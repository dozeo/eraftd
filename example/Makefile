run: clean build
	./raftd -p 4001 /tmp/raftd/node.1

cluster: stop clean build
	./raftd -v -p 4001 -join localhost:4000 /tmp/raftd/node.1 &
	sleep 1
	./raftd -v -p 4002 -join localhost:4001 /tmp/raftd/node.2 &
	sleep 1
	./raftd -v -p 4003 -join localhost:4001 /tmp/raftd/node.3 &

bigcluster: cluster
	sleep 1
	./raftd -v -p 4004 -join localhost:4001 /tmp/raftd/node.4 &
	sleep 1
	./raftd -v -p 4005 -join localhost:4001 /tmp/raftd/node.5 &

build:
	go build -o raftd

stop: realstop clean

realstop:
	killall raftd || true

clean:
	if [ -d /tmp/raftd/node.1 ] ; then rm -r /tmp/raftd ; fi
