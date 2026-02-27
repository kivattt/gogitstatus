if [ -z $1 ]; then
	echo "Usage: $0 [path to git repo]"
	echo
	echo "Meant to be used on a big repo like chromium."
	echo "Outputs CSV format to stderr"
	exit
fi

if [ ! -x ./showstatus ]; then
	echo "unable to find ./showstatus"
	echo
	echo "Remember to run 'go build'"
	exit
fi

output_timeout_and_real_time_spent()
{
	timeout=$1
	repository_path=$2
	/usr/bin/time --quiet -f "%e,$timeout" ./showstatus --timeout=$timeout "$repository_path" > /dev/null
}

repository_path=$1

echo "Real time spent (s),Timeout (ms)" 1>&2

# 0 up to 10000ms in intervals of 16ms.
# Automatically stops when the timeout exceeds the runtime
for timeout in {0..10000..16}
do
	output_timeout_and_real_time_spent $timeout $repository_path
	if [ $? -ne 2 ]; then
		break
	fi
done
