# Mini-Ping
##### Author: Muthu Arivoli
##### Email: ma381@duke.edu
This is a basic implementation of the ping utility. IPv4 and IPv6 are both supported. Examples of usage include:

The following command pings 8.8.8.8 forever, with a one second interval between pings.
```
mini-ping 8.8.8.8
```

The following command pings google 10 times before stopping, again with a one second interval.
```
mini-ping -c 10 www.google.com
```

The following command pings the IPv6 address of google with a 64 byte payload (72 bytes overall) with an interval of 0.5 seconds.
```
mini-ping -s 64 -i 0.5 2001:4860:4860::8888
```

## Usage
**mini-ping** [ **-c count** ] [ **-i interval** ] [ **-s packetsize** ] [ **-t ttl** ] [ **-w deadline** ]  **destination**

-c count

:   Stop after sending *count* packets.

-i interval

:   Wait *interval* seconds between sending each packet. The default is to wait for one second between each packet normally

-s packetsize

:   Specifies the number of data bytes to be sent. The default is 56, which translates into 64 ICMP data bytes when combined with the 8 bytes of ICMP header data.

-t ttl

:   Set the IP Time to Live.


-w deadline

:   Specify a timeout, in seconds, before ping exits regardless of how many packets have been sent or received.





## Build
To build this project, please ensure that you have installed Go. This project has been tested using Go v1.14.2 on Ubuntu. Additional packages necessary include the extra networking features, which may be installed using 

```
go get golang.org/x/net/ipv4       
go get golang.org/x/net/ipv6
go get golang.org/x/net/icmp
```

After this, you can build using 

```
go build mini-ping.go
```

## Bugs
The reported values for TTL on Windows are currently inaccurate (they always report zero). This is due to the control flags in Go not being able to be set on Windows (since it has not been implemented for Windows in the Go library yet).
