package main

import (
	"flag"
	"fmt"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"math"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type MiniPinger struct {
	ipAddress *net.IPAddr
	count int
	ttl int
	interval time.Duration
	packetSize int
	deadline time.Duration
	packetsReceived int
	packetsSent int
	timeSent map[int]time.Time
	travelTimes []time.Duration
	startTime time.Time
	finished chan bool
}

// Creates a new mini-pinger
func NewMiniPinger(input string, count int, ttl int, interval time.Duration, packetSize int, deadline time.Duration) (*MiniPinger,error) {
	mp := new(MiniPinger)
	ipAddress,err := net.ResolveIPAddr("ip", input)
	if err!=nil {
		return nil,err
	}
	mp.ipAddress = ipAddress
	mp.count = count
	mp.ttl = ttl
	mp.interval = interval
	mp.packetSize = packetSize
	mp.deadline = deadline
	mp.finished = make(chan bool, 2)
	mp.packetsSent = 0
	mp.packetsReceived = 0
	mp.timeSent = make(map[int]time.Time)
	mp.travelTimes = make([]time.Duration,0)
	return mp,nil
}

// Returns the network type depending on whether the address is ipv4 or ipv6
func (mp *MiniPinger) getNetwork() string {
	if mp.ipAddress.IP.To4() != nil {
		return "ip4:icmp"
	}else{
		return "ip6:ipv6-icmp"
	}
}

// main function that starts and maintains all processes
func (mp *MiniPinger) run(wgMain *sync.WaitGroup) {
	defer wgMain.Done()
	networkType := mp.getNetwork()
	conn, err := icmp.ListenPacket(networkType, "::")
	if err!=nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	if mp.ipAddress.IP.To4() != nil {
		conn.IPv4PacketConn().SetControlMessage(ipv4.FlagTTL, true)
		conn.IPv4PacketConn().SetTTL(mp.ttl)
	} else{
		conn.IPv6PacketConn().SetControlMessage(ipv6.FlagHopLimit, true)
		conn.IPv6PacketConn().SetHopLimit(mp.ttl)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go mp.receivePacket(conn,&wg)
	go mp.checkFinish(&wg)

	ticker := time.NewTicker(mp.interval)
	defer ticker.Stop()

	for{
		select{
		case <-mp.finished:
			wg.Wait()
			return
		case <-ticker.C:
			mp.sendPacket(conn)
		}
	}
}

// Sends a packet
func (mp *MiniPinger) sendPacket(conn *icmp.PacketConn)error{
	var mType icmp.Type
	if mp.ipAddress.IP.To4() != nil {
		mType = ipv4.ICMPTypeEcho
	}else{
		mType = ipv6.ICMPTypeEchoRequest
	}
	message := icmp.Message{
		Type:     mType,
		Code:     0,
		Body:     &icmp.Echo{
			ID:   os.Getpid(),
			Seq:  mp.packetsSent,
			Data: make([] byte, mp.packetSize),
		},
	}
	mp.timeSent[mp.packetsSent] = time.Now()
	b,err := message.Marshal(nil)
	if err!=nil {
		return err
	}
	_, err = conn.WriteTo(b,mp.ipAddress)
	mp.packetsSent++
	return err
}

// Receive and process a packet
func (mp *MiniPinger) receivePacket (conn *icmp.PacketConn, wg *sync.WaitGroup){
	defer wg.Done()
	for {
		select {
		case <-mp.finished:
			return
		default:
			conn.SetReadDeadline(time.Now().Add(mp.interval))
			reply := make([]byte, mp.packetSize+100)
			var ttl int
			var err error
			var icmpCode int
			var numBytes int
			if mp.ipAddress.IP.To4() != nil {
				var controlMessage *ipv4.ControlMessage
				numBytes, controlMessage, _, err = conn.IPv4PacketConn().ReadFrom(reply)
				if err == nil && controlMessage != nil {
					ttl = controlMessage.TTL
				}
				icmpCode = 1
			} else {
				var controlMessage *ipv6.ControlMessage
				numBytes, controlMessage, _, err = conn.IPv6PacketConn().ReadFrom(reply)
				if err == nil && controlMessage != nil {
					ttl = controlMessage.HopLimit
				}
				icmpCode = 58
			}
			rm, err := icmp.ParseMessage(icmpCode, reply)
			if err != nil {
				fmt.Println("Error parsing message")
			}
			switch rm.Body.(type) {
			case *icmp.Echo:
				messageBody := rm.Body.(*icmp.Echo)
				if messageBody.ID != os.Getpid() {
					continue
				}
				packetNumber := messageBody.Seq
				travelTime := time.Now().Sub(mp.timeSent[packetNumber])
				mp.travelTimes = append(mp.travelTimes, travelTime)
				fmt.Printf("%d bytes from %s: icmp_seq=%d time=%v ttl=%v \n",
					numBytes, mp.ipAddress, packetNumber, travelTime, ttl)
				mp.packetsReceived++
			default:
				fmt.Println("Request timed out.")
			}
		}

	}
}

// Checks if any of the terminating conditions have been met
func (mp *MiniPinger) checkFinish(wg *sync.WaitGroup){
	defer wg.Done()

	startTime := time.Now()
	mp.startTime = startTime
	endTime := startTime.Add(mp.deadline)
	for {
		currTime := time.Now()
		select{
		case <-mp.finished:
			return
		default:
			if currTime.After(endTime) {
				close(mp.finished)
				return
			}
			if mp.packetsSent>mp.count {
				close(mp.finished)
				return
			}
		}
	}
}

// Prints the overall statistics of the current run
func (mp *MiniPinger) printStats(){
	if mp.packetsSent==0 {
		return
	}
	loss := 100-100*mp.packetsReceived/mp.packetsSent
	min := math.MaxFloat32
	max := -1.0
	avg := float64(time.Duration(0))
	for _,value := range mp.travelTimes{
		if min > float64(value) {
			min = float64(value)
		}
		if max < float64(value) {
			max = float64(value)
		}
		avg += float64(value)
	}
	avg/=float64(len(mp.travelTimes))
	fmt.Printf("%d packets transmitted, %d packets received, %d%% loss, time %d ms \n",
		mp.packetsSent, mp.packetsReceived, loss, time.Now().Sub(mp.startTime)/time.Millisecond)
	if mp.packetsReceived>0 {
		fmt.Printf("rtt min/max/avg: %f/%f/%f ms\n",
			 min/1000000, max/1000000, avg/1000000)
	}
	return
}

func main() {
	count := flag.Int("c", math.MaxInt32, "number of packets to send until stopping")
	ttl := flag.Int("t", 128, "time to live")
	intervalFloat := flag.Float64("i", 1, "time between consecutive pings in seconds")
	packetSize := flag.Int("s",56, "number of bytes to send")
	deadlineInteger := flag.Float64("w", math.MaxInt32, "time until stopping")
	flag.Parse()
	ipAddr := flag.Arg(0)
	interval := time.Duration(int(*intervalFloat*1000)) * time.Millisecond
	deadline := time.Duration(int(*deadlineInteger*1000)) * time.Millisecond
	mp, err := NewMiniPinger(ipAddr,*count,*ttl,interval,*packetSize,deadline)
	if err!=nil {
		fmt.Println("ERROR encountered")
		return
	}
	ctrlc := make(chan os.Signal)
	signal.Notify(ctrlc,os.Interrupt,syscall.SIGTERM)
	go func() {
		<-ctrlc
		close(mp.finished)
		return
	}()
	var wgMain sync.WaitGroup
	wgMain.Add(1)
	mp.run(&wgMain)
	wgMain.Wait()
	mp.printStats()
}