package main

import "core:fmt"
import "base:intrinsics"
import "core:math/rand"
import rl "vendor:raylib"

//DEBUG :: true
debug := false

WIDTH :: 1280
HEIGHT :: 720

WONTEXTURE_WIDTH :: 600
WONTEXTURE_HEIGHT :: 300

CELL_SIZE :: 41

BOMB_IMAGE :: #load("bomb.png")
FLAG_IMAGE :: #load("flag.png")
WON_IMAGE  :: #load("won.png")

// From htwins.net/minesweeper
// var numberColors = [tileColor, "#0040FF","#008000","#FF0000","#000080","#800080","#008080","#606060","#000000"];
NUMBER_COLORS := [8]rl.Color{
	{0x00, 0x40, 0xFF, 0xFF}, // 1
	{0x00, 0x80, 0x00, 0xFF}, // 2
	{0x75, 0x00, 0x00, 0xFF}, // 3
	{0x00, 0x00, 0x80, 0xFF}, // 4
	{0x80, 0x00, 0x80, 0xFF}, // 5
	{0x00, 0x80, 0x80, 0xFF}, // 6
	{0x60, 0x60, 0x60, 0xFF}, // 7
	{0x00, 0x00, 0x00, 0xFF}, // 8
}

Theme :: struct {
	exposed: rl.Color,
	border: rl.Color,
	borderLighter: rl.Color,
	unexposed: rl.Color,
}

defaultTheme := Theme {
	exposed = {40, 40, 40, 255},
	border = {90, 90, 90, 255},
	unexposed = {100, 100, 102, 255},
}

lightTheme := Theme {
	border = {90, 90, 90, 255},
	borderLighter = {30, 30, 30, 255},
	unexposed = {40, 40, 40, 255},
	exposed = {130, 130, 132, 255},
}

/*lightTheme := Theme {
	exposed = {200, 200, 200, 255},
	border = {20, 20, 20, 255},
}*/

Cell :: struct {
	exposed: bool,
	isFlag: bool,
	isBomb: bool,
	numBombsSurrounding: int,
}

Game :: struct {
	gameInProgress: bool,
	gameOver: bool,
	gameWon: bool,
	seed: rand.Xoshiro256_Random_State,
	bombConcentration: f64,
	hoveredIndex: int,
	grid: [dynamic]Cell,
	width: int,
	height: int,
	lastRightMouseDown: bool,
	lastRightClickedWasFlagged: bool,
	earthQuakeBuffer: [dynamic]int, // List of visited indices
}

// FIXME: Replace bombConcentration with bombCount (?)
generate_random_grid :: proc(g: ^Game) {
	g.gameInProgress = true
	g.gameOver = false

	assert(g.grid != nil)
	assert(g.width > 0)
	assert(g.height > 0)

	gen := rand.xoshiro256_random_generator(&g.seed)

	length := g.width * g.height
	// Place bombs
	for i := 0; i < length; i += 1 {
		if i == g.hoveredIndex do continue // Don't place a bomb on the clicked location

		g.grid[i] = Cell{
			exposed = false,
			isBomb = rand.float64(gen) <= g.bombConcentration,
			numBombsSurrounding = i,
		}
	}

	update_bomb_counts(g)
}

have_we_won :: proc(g: ^Game) -> bool {
	length := g.width * g.height
	for i := 0; i < length; i += 1 {
		if g.grid[i].isBomb do continue

		if !g.grid[i].exposed do return false
	}

	return true
}

xy_to_index :: proc(g: Game, x, y: i32) -> int {
	return int(x + y * i32(g.width))
}

index_to_xy :: proc(g: Game, index: int) -> (x, y: i32) {
	x = i32(index % g.width)
	y = i32(index / g.width)
	return x, y
}

in_bounds :: proc(g: Game, x, y: i32) -> bool {
	if x < 0 || y < 0 do return false
	if x >= i32(g.width) || y >= i32(g.height) do return false
	return true
}

update_bomb_counts :: proc(g: ^Game) {
	length := g.width * g.height
	for i := 0; i < length; i += 1 {
		x, y := index_to_xy(g^, i)

		numBombsSurrounding := 0
		for dx: i32 = -1; dx <= 1; dx += 1 {
			for dy: i32 = -1; dy <= 1; dy += 1 {
				neighborX := x + dx
				neighborY := y + dy

				if in_bounds(g^, neighborX, neighborY) {
					index := xy_to_index(g^, neighborX, neighborY)
					if g.grid[index].isBomb {
						numBombsSurrounding += 1
					}
				}
			}
		}

		g.grid[i].numBombsSurrounding = numBombsSurrounding
	}
}

draw_grid :: proc(g: ^Game, theme: Theme, bombTexture, flagTexture, wonTexture: rl.Texture, xOffset, yOffset: i32) {
	defer {
		if debug {
			if g.hoveredIndex != -1 {
				x, y := index_to_xy(g^, g.hoveredIndex)
				xPos := xOffset + i32(CELL_SIZE) * x
				yPos := yOffset + i32(CELL_SIZE) * y
				rl.DrawRectangle(xPos - 1, yPos - 1, CELL_SIZE, CELL_SIZE, {0, 255, 0, 50})
			}
		}
	}

	length := g.width * g.height
	for i := 0; i < length; i += 1 {
		x, y := index_to_xy(g^, i)

		exposed := g.grid[i].exposed
		isFlag := g.grid[i].isFlag
		isBomb := g.grid[i].isBomb
		numBombsSurrounding := g.grid[i].numBombsSurrounding

		color := theme.exposed

		cellSize := i32(CELL_SIZE)
		// Outer
		xPos := xOffset + x * cellSize
		yPos := yOffset + y * cellSize

		if exposed {
			rl.DrawRectangle(xPos, yPos, cellSize, cellSize, theme.border)
		} else {
			rl.DrawRectangle(xPos, yPos, cellSize, cellSize, theme.borderLighter)
		}

		if g.gameOver {
			oldXPos := xPos
			oldYPos := yPos
			// Inner
			if exposed {
				xPos += 1
				yPos += 1
				if !isBomb {
					rl.DrawRectangle(xPos, yPos, cellSize-2, cellSize-2, color)
				}

				xPos = xOffset + x * cellSize + cellSize / 2 - 5
				yPos = yOffset + y * cellSize + cellSize / 2 - 6
				if !isBomb && numBombsSurrounding > 0 {
					text := fmt.ctprintf("{}", numBombsSurrounding)
					rl.DrawText(text, xPos, yPos, 20, NUMBER_COLORS[numBombsSurrounding-1])
				}
			} else {
				rl.DrawRectangle(xPos, yPos, cellSize-2, cellSize-2, theme.unexposed)
				if isFlag {
					rl.DrawTexture(flagTexture, xPos, yPos, {255,255,255,255})
				}
			}

			if isBomb {
				// Game over bomb drawing opacity
				rl.DrawTexture(bombTexture, oldXPos+1, oldYPos+1, {255,255,255, 255})
				if isFlag {
					rl.DrawTexture(flagTexture, xPos, yPos, {255,255,255,255})
				}
			}
		} else {
			// Inner
			if exposed {
				xPos += 1
				yPos += 1
				if isBomb {
					rl.DrawTexture(bombTexture, xPos, yPos, {255,255,255,255})
				} else {
					rl.DrawRectangle(xPos, yPos, cellSize-2, cellSize-2, color)
				}

				xPos = xOffset + x * cellSize + cellSize / 2 - 5
				yPos = yOffset + y * cellSize + cellSize / 2 - 6
				if !isBomb && numBombsSurrounding > 0 {
					text := fmt.ctprintf("{}", numBombsSurrounding)
					rl.DrawText(text, xPos, yPos, 20, NUMBER_COLORS[numBombsSurrounding-1])
				}
			} else {
				rl.DrawRectangle(xPos, yPos, cellSize-2, cellSize-2, theme.unexposed)
				if isFlag {
					rl.DrawTexture(flagTexture, xPos, yPos, {255,255,255,255})
				}
			}
		}

		if g.gameWon {
			width := rl.GetScreenWidth()
			height := rl.GetScreenHeight()
			rl.BeginBlendMode(.ALPHA)
			halfBoardWidth: i32 = i32(CELL_SIZE * g.width / 2)
			halfBoardHeight: i32 = i32(CELL_SIZE * g.height / 2)
			rl.DrawTexture(wonTexture, xOffset + halfBoardWidth - WONTEXTURE_WIDTH/2, yOffset + halfBoardHeight - WONTEXTURE_HEIGHT/2, {255,255,255,100})
			rl.EndBlendMode()
		}
	}
}

mouse_pos_to_xy :: proc(mouseX, mouseY: f32, xOffset, yOffset: int) -> (x, y: i32) {
	mouseX := mouseX - f32(xOffset)
	mouseY := mouseY - f32(yOffset)
	return i32(mouseX / f32(CELL_SIZE)), i32(mouseY / f32(CELL_SIZE))
}

handle_input :: proc(g: ^Game, xOffset, yOffset: int) {
	if rl.IsKeyPressed(.LEFT_SHIFT) {
		debug = !debug
	}

	if g.gameOver || g.gameWon {
		return
	}

	mouse := rl.GetMousePosition()
	g.hoveredIndex = -1
	if int(mouse[0]) >= xOffset && int(mouse[1]) >= yOffset {
		x, y := mouse_pos_to_xy(mouse[0], mouse[1], xOffset, yOffset)
		if in_bounds(g^, x, y) {
			g.hoveredIndex = xy_to_index(g^, x, y)
		}
	}

	if g.hoveredIndex != -1 {
		isRightDown := rl.IsMouseButtonDown(.RIGHT)

		if isRightDown && !g.lastRightMouseDown {
			g.lastRightClickedWasFlagged = g.grid[g.hoveredIndex].isFlag
		}

		if isRightDown {
			flag_cell(g, g.hoveredIndex, !g.lastRightClickedWasFlagged)
		} else if rl.IsMouseButtonDown(.LEFT) {
			if !g.gameInProgress {
				generate_random_grid(g)
			}
			expose_cell(g, g.hoveredIndex)
			if have_we_won(g) {
				g.gameWon = true
				g.gameInProgress = false
			}
			fmt.println(g.hoveredIndex)
		}

		g.lastRightMouseDown = isRightDown
	}
}

count_flags_surrounding :: proc(g: Game, index: int) -> (numFlags: int) {
	x, y := index_to_xy(g, index)
	for dx: i32 = -1; dx <= 1; dx += 1 {
		for dy: i32 = -1; dy <= 1; dy += 1 {
			if dx == 0 && dy == 0 do continue // Don't count ourselves...

			if in_bounds(g, x+dx, y+dy) {
				i := xy_to_index(g, x+dx, y+dy)
				if g.grid[i].isFlag {
					numFlags += 1
				}
			}
		}
	}

	return numFlags
}

expose_around_zero_cells_earthquake :: proc(g: ^Game, index: int) {
	g.earthQuakeBuffer = make([dynamic]int, 0)
	bufferCursor := 0

	x, y := index_to_xy(g^, index)
	for dx: i32 = -1; dx <= 1; dx += 1 {
		for dy: i32 = -1; dy <= 1; dy += 1 {
			if dx == 0 && dy == 0 do continue // Don't count ourselves

			for _, e in g.earthQuakeBuffer {
				// TODO: Continue here...
			}
		}
	}

	// FIXME: Reuse and grow the buffer instead of re-allocating every time
	delete(g.earthQuakeBuffer)
}

expose_around_zero_cells_recursively :: proc(g: ^Game, index: int) {
	cell := g.grid[index]
	/*if cell.exposed || cell.numBombsSurrounding != 0 {
		return
	}*/

	if cell.numBombsSurrounding != 0 {
		return
	}

	x, y := index_to_xy(g^, index)
	for dx: i32 = -1; dx <= 1; dx += 1 {
		for dy: i32 = -1; dy <= 1; dy += 1 {
			if dx == 0 && dy == 0 do continue // Don't expose ourselves...

			i := xy_to_index(g^, x+dx, y+dy)
			g.grid[i].exposed = true
		}
	}

	for dx: i32 = -1; dx <= 1; dx += 1 {
		for dy: i32 = -1; dy <= 1; dy += 1 {
			if dx == 0 && dy == 0 do continue // Don't expose ourselves...

			i := xy_to_index(g^, x+dx, y+dy)
			expose_around_zero_cells_recursively(g, i)
			//g.grid[i].exposed = true
		}
	}
}

expose_cell :: proc(g: ^Game, index: int) {
	if g.grid[index].isFlag {
		return
	}

	// Clicked an exposed cell, let's recursively attempt to clear around it
	if g.grid[index].exposed {
		// Do nothing when clicking on an exposed bomb
		if g.grid[index].isBomb {
			return
		}

		numFlags := count_flags_surrounding(g^, index)
		// Not enough flags to clear automatically
		if numFlags < g.grid[index].numBombsSurrounding {
			return
		}

		/*if g.grid[index].numBombsSurrounding == 0 {
			expose_around_zero_cells_recursively(g, index)
		}*/

		x, y := index_to_xy(g^, index)
		for dx: i32 = -1; dx <= 1; dx += 1 {
			for dy: i32 = -1; dy <= 1; dy += 1 {
				if dx == 0 && dy == 0 do continue // Don't include ourselves...

				if in_bounds(g^, x+dx, y+dy) {
					i := xy_to_index(g^, x+dx, y+dy)

					if !g.grid[i].isFlag {
						g.grid[i].exposed = true
						if g.grid[i].isBomb {
							g.gameOver = true
						}
						//expose_cell(g, i)
						//expose_around_zero_cells_recursively(g, index)
						expose_around_zero_cells_earthquake(g, index)
					}
				}
				//expose_cell(g, i)
			}
		}
	} else {
		if g.grid[index].isBomb {
			g.gameOver = true
		}

		g.grid[index].exposed = true
	}

	// Fun 5x5 brush thing
	/*x, y := index_to_xy(g^, index)

	for dx: i32 = -2; dx <= 2; dx += 1 {
		for dy: i32 = -2; dy <= 2; dy += 1 {
			xPos := x + dx
			yPos := y + dy

			if in_bounds(g^, xPos, yPos) {
				i := xy_to_index(g^, i32(xPos), i32(yPos))
				if !g.grid[i].isFlag {
					g.grid[i].exposed = true
				}
			}
		}
	}*/
}

flag_cell :: proc(g: ^Game, index: int, b: bool) {
	if !g.grid[index].exposed {
		g.grid[index].isFlag = b
	}
}

reset_grid :: proc(g: ^Game) {
	g.gameInProgress = false
	g.gameOver = false
	g.gameWon = false

	length := g.width * g.height
	for i := 0; i < length; i += 1 {
		g.grid[i] = Cell{}
	}
}

main :: proc() {
	rl.SetConfigFlags({.WINDOW_RESIZABLE})
	rl.InitWindow(WIDTH, HEIGHT, "mysweeper")
	defer rl.CloseWindow()
	rl.SetTargetFPS(rl.GetMonitorRefreshRate(rl.GetCurrentMonitor()))
	//rl.SetWindowMinSize(...)

	bombImage := rl.LoadImageFromMemory(".png", raw_data(BOMB_IMAGE), i32(len(BOMB_IMAGE)))
	bombTexture := rl.LoadTextureFromImage(bombImage)

	flagImage := rl.LoadImageFromMemory(".png", raw_data(FLAG_IMAGE), i32(len(FLAG_IMAGE)))
	flagTexture := rl.LoadTextureFromImage(flagImage)

	wonImage := rl.LoadImageFromMemory(".png", raw_data(WON_IMAGE), i32(len(WON_IMAGE)))
	wonTexture := rl.LoadTextureFromImage(wonImage)

	theme := lightTheme

	grid := Game{
		width = 16,
		height = 16,
	}
	grid.grid = make([dynamic]Cell, grid.width*grid.height)
	defer delete(grid.grid)

	// Seed the RNG
	for i := 0; i < 4; i += 1 {
		grid.seed.s[i] = u64(intrinsics.read_cycle_counter())
	}
	grid.bombConcentration = 0.1
	reset_grid(&grid)
	//generate_random_grid(&grid, &randomSeed, bombConcentration)

	for !rl.WindowShouldClose() {
		rl.BeginDrawing()
		rl.ClearBackground({20, 20, 22, 255})

		if rl.IsKeyDown(.Q) || rl.IsKeyDown(.ESCAPE) || rl.IsKeyDown(.CAPS_LOCK) {
			break
		}

		if rl.IsKeyPressed(.R) || rl.IsMouseButtonPressed(.EXTRA) {
			reset_grid(&grid)
		}

		xOffsetGrid := 15
		yOffsetGrid := 15
		handle_input(&grid, xOffsetGrid, yOffsetGrid)
		draw_grid(&grid, theme, bombTexture, flagTexture, wonTexture, i32(xOffsetGrid), i32(yOffsetGrid))

		if debug {
			rl.DrawFPS(0, 0)
		}

		rl.EndDrawing()
	}
}
