package compositions

import (
	"encoding/json"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"log"
	"os"
	"time"
)

type Node struct {
	IP string
	Port string
}

type NodeStats struct {
	SrcCount int
	DstCount int
}

type NetCaptureConfig struct {
	Device string `json:"device"`
	Snapshot_len int `json:"snapshot_len"`
	Promiscuous bool `json:"promiscuous"`
}

type NetCapture struct {
	handle *pcap.Handle
	config NetCaptureConfig
	capture_channel chan int
	Stats map[string]NodeStats
}

func InitializeCapture(config_file string, timeout time.Duration) (*NetCapture, error) {
	file, err := os.Open(config_file)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	config := NetCaptureConfig{}
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	handle, err := pcap.OpenLive(config.Device, int32(config.Snapshot_len), config.Promiscuous, timeout)
	if err != nil {
		return nil, err
	}

	n := &NetCapture{handle, config, make(chan int), map[string]NodeStats{}}
	return n, nil
}

func getNodeString(ip string, port string) string {
	return ip + ":" + port
}

func (n *NetCapture) processPacket(packet gopacket.Packet) {
	var srcAddr string
	var dstAddr string
	var srcPort string
	var dstPort string

	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer != nil {
		ip, _ := ipLayer.(*layers.IPv4)
		srcAddr = ip.SrcIP.String()
		dstAddr = ip.DstIP.String()
	}

	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer != nil {
		tcp, _ := tcpLayer.(*layers.TCP)
		srcPort = tcp.SrcPort.String()
		dstPort = tcp.DstPort.String()
	}

	src := getNodeString(srcAddr, srcPort)
	dst := getNodeString(dstAddr, dstPort)

	if ns, ok := n.Stats[src]; ok {
		ns.SrcCount += 1
		n.Stats[src] = ns
	} else {
		ns := NodeStats{1,0}
		n.Stats[src] = ns
	}

	if ns, ok := n.Stats[dst]; ok {
		ns.DstCount += 1
		n.Stats[dst] = ns
	} else {
		ns := NodeStats{0, 1}
		n.Stats[dst] = ns
	}
}

func (n *NetCapture) ProcessPackets() {
	log.Println("Starting capture")
	packetSource := gopacket.NewPacketSource(n.handle, n.handle.LinkType())
	packet_chan := packetSource.Packets()
	for {
		select {
			case <-n.capture_channel:
				log.Println("Finishing capturing packets")
				return
			case packet := <-packet_chan:
				log.Println("Got packet")
				n.processPacket(packet)
			default:
				continue
		}
	}
}

func (n *NetCapture) StartCapture() {
	go n.ProcessPackets()
}

func (n *NetCapture) StopCapture() {
	n.capture_channel <- 0
	n.handle.Close()
}
