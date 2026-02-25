package decoding

// See the bottom of this file for the C++ (Intel intrinsics) implementation of the meat of the SIMD

import "core:strings"
import "core:simd"
import "base:intrinsics"
import slice "core:slice"

@(private="file")
BYTES_PER_ITERATION :: 16

@(private)
hex_to_index :: proc(b: u8) -> int {
    switch b {
        case '0'..='9':
            return int(b - '0')
        case 'a'..='f':
            return int(b - 'a' + 10)
        case 'A'..='F':
            return int(b - 'A' + 10)
    }
    
    return -1
}

hex_decode :: proc(bytes: []byte) -> ([dynamic]u8, Error) {
    length := len(bytes) - (len(bytes) & 1)
    sb: strings.Builder
    strings.builder_init_len_cap(&sb, 0, length / 2)
    
    i: int
    data: #simd[BYTES_PER_ITERATION]u8
    for i = 0; i < length - BYTES_PER_ITERATION; i += BYTES_PER_ITERATION {
        #no_bounds_check {
            data = simd.from_slice(#simd[BYTES_PER_ITERATION]u8, slice.from_ptr(&bytes[i], BYTES_PER_ITERATION))
        }
        data = simd.bit_or(data, 0b00100000) // Case-insensitive
        
        // TODO: Look into simd.clamp() or min + max functions

        // '0' through '9'
        a := simd.saturating_sub(data, '0' - 1)
        aMask := simd.lanes_lt(a, 10 + 1)
        a = simd.bit_and(a, aMask)

        // 'a' through 'f', case-insensitive
        b := simd.saturating_sub(data, 'a' - 1)
        bMask := simd.lanes_lt(b, 6 + 1)
        b = simd.bit_and(b, bMask)

        b = simd.bit_or(a, b)
        invalid := simd.reduce_or(simd.lanes_eq(b, 0)) != 0
        if invalid {
            break
        }

        // Get the value of 'a' through 'f', starting at 11
        b = simd.saturating_sub(data, 'a' - 11)
        b = simd.bit_or(a, b)
        b = simd.sub(b, 1)

        left := simd.shuffle(b, b, 0, 2, 4, 6, 8, 10, 12, 14)
        left = simd.shl(left, 4)
        right := simd.shuffle(b, b, 1, 3, 5, 7, 9, 11, 13, 15)

        result := simd.bit_or(left, right)
        
        #no_bounds_check {
            resize(&sb.buf, len(sb.buf) + BYTES_PER_ITERATION/2)
            intrinsics.unaligned_store(cast(^#simd[BYTES_PER_ITERATION/2]u8)&sb.buf[len(sb.buf) - BYTES_PER_ITERATION/2], result)
        }
    }

    // Last n <= 16 bytes
    remaining := min(BYTES_PER_ITERATION, length - i)
    
    for j := 0; j < remaining; j += 2 {
        a := hex_to_index(bytes[j+i + 0])
        b := hex_to_index(bytes[j+i + 1])
        
        if (a | b) < 0 {
            break
        }
        
        strings.write_byte(&sb, u8(a << 4 | b))
    }
    
    if len(sb.buf) == 0 {
        return sb.buf, .Failed
    }
    
    if len(sb.buf) != length / 2 || len(bytes) & 1 == 1 {
        return sb.buf, .Partial_Decode
    }
    
    return sb.buf, nil
}

/*
std::string d(std::string &s) {
    __m128i str = _mm_loadu_epi8(s.data());

    // '0' through '9'
    __m128i toSubtract = _mm_set1_epi8(47);
    __m128i max = _mm_max_epi8(str, toSubtract);
    __m128i a = _mm_sub_epi8(max, toSubtract);
    //__m128i a = _mm_subs_epi8(str, _mm_set1_epi8(47));
    __m128i aMask = _mm_cmplt_epi8(a, _mm_set1_epi8(11));
    a = _mm_and_si128(a, aMask);

    // 'a' through 'f', case-insensitive
    __m128i bInsensitive = _mm_or_si128(str, _mm_set1_epi8(0b00100000)); // Mixed-case
    toSubtract = _mm_set1_epi8(96);
    max = _mm_max_epi8(bInsensitive, toSubtract);
    __m128i b = _mm_sub_epi8(max, toSubtract);
    //__m128i b = _mm_subs_epi8(bInsensitive, _mm_set1_epi8(96));
    __m128i bMask = _mm_cmplt_epi8(b, _mm_set1_epi8(7));
    b = _mm_and_si128(b, bMask);

    __m128i c = _mm_or_si128(a, b);
    int check = _mm_movemask_epi8(_mm_cmpeq_epi8(c, _mm_set1_epi8(0)));
    if (check != 0) {
        std::cerr << "Invalid hex!\n";
        return s;
    }

    toSubtract = _mm_set1_epi8(96 - 10);
    max = _mm_max_epi8(bInsensitive, toSubtract);
    b = _mm_sub_epi8(max, toSubtract);
    //b = _mm_subs_epi8(bInsensitive, _mm_set1_epi8(96 - 10));
    c = _mm_or_si128(a, b);
    c = _mm_sub_epi8(c, _mm_set1_epi8(1));

    __m128i left = _mm_shuffle_epi8(c, _mm_setr_epi8(
        0, 2, 4, 6, 8, 10, 12, 14,
        0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80 // don't care about these, might aswell set to zero
    ));
    left = _mm_slli_epi16(left, 4); // left-shift 4 bits
    __m128i right = _mm_shuffle_epi8(c, _mm_setr_epi8(
        1, 3, 5, 7, 9, 11, 13, 15,
        0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80 // don't care about these, might aswell set to zero
    ));

    __m64 result = _mm_cvtsi64_m64(_mm_cvtsi128_si64(_mm_or_si128(left, right)));
    _mm_stream_pi((__m64*)s.data(), result);
    //_mm_storeu_epi8(s.data(), result); //

    return s;
}
*/