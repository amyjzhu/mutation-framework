package compositions

import (
	"encoding/json"
	"errors"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"
)

var NetworkLibraryDatabase map[string]bool = map[string]bool{
	"net": true,
	"http": true,
	"rpc": true,
	"jsonrpc": true,
	"cgi": true,
	"cookiejar": true,
	"fcgi": true,
	"httptest": true,
	"httptrace": true,
	"httputil": true,
	"pprof": true,
	"mail": true,
	"smtp": true,
	"url": true,
	"textproto": true,
	"iptables": true,
	"ipvs": true,
	"utilnet": true,
	"utiliptables": true,
}

type FileInfo struct {
	Path string `json:"Path"`
	StartLine int `json:"StartLine"`
	EndLine int `json:"EndLine`
}

type Address struct {
	IP string `json:"IP"`
	Port string `json:"Port"`
}

type NodeRole struct {
	Name string `json:"Name"`
	Addresses []Address `json:"Address"`
	SourceCode []FileInfo `json:"SourceCode"`
}

type RoleInfo struct {
	NumFiles int
	NumLines int
	NumAddresses int
	NetworkRatio float64
}

type Roles struct {
	R []NodeRole `json:"Role"`
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

func (nr *NodeRole) GetTotalLines() int {
	num_lines := 0
	for _, source_code := range nr.SourceCode {
		num_lines = num_lines + (source_code.EndLine - source_code.StartLine)
	}
	return num_lines
}

func (nr *NodeRole) GetTotalFiles() int {
	m := make(map[string]bool)
	for _, source_code := range nr.SourceCode {
		if _, ok := m[source_code.Path]; !ok {
			m[source_code.Path] = true
		}
	}
	return len(m)
}

func (nr *NodeRole) CalculateNetworkRatio() float64 {
	fset := token.NewFileSet()
	m := make(map[string]bool)	
	count := 0
	callCount := 0
	networkCount := 0
	for _, source_code := range nr.SourceCode {
		filepath := source_code.Path
		gopath := os.Getenv("GOPATH")
		full_path := path.Join(gopath, "src", filepath)
		file_node, err := parser.ParseFile(fset, full_path, nil, parser.ParseComments)
		if err != nil {
			log.Println("Something went wrong trying to parse file", full_path)
			continue
		}
		ast.Inspect(file_node, func(n ast.Node) bool {
			count += 1
			exp, ok := n.(*ast.CallExpr)
			if ok {
				if fun, ok := exp.Fun.(*ast.SelectorExpr); ok {
					switch fun.X.(type) {
					case *ast.Ident:
						if pack, ok := fun.X.(*ast.Ident); ok {
							funcName := pack.Name
							m[funcName] = true
							if _, ok := NetworkLibraryDatabase[funcName]; ok {
								networkCount += 1
							}
						}
					}
                }
				callCount += 1
			}
			return true
		})
	}
	var net_ratio float64
	net_ratio = float64(networkCount) / float64(callCount)
	return net_ratio
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

func InitializeNodeRoles(config_file string) (*Roles, error) {
	file, err := os.Open(config_file)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var result Roles
	json.Unmarshal(data, &result)
	return &result, nil
}

func (nr *Roles) GetNumNodeRoles() int {
	return len(nr.R)
}

func (nr *Roles) GetNodeRoleNames() []string {
	var names []string
	for _ , r := range nr.R {
		names = append(names, r.Name)
	}
	return names
}

func (nr *Roles) GetNodeStaticAnalysisInfo(role_name string) (*RoleInfo, error) {
	for _, r := range nr.R {
		if r.Name == role_name {
			num_files := r.GetTotalFiles()
			num_lines := r.GetTotalLines()
			num_addresses := len(r.Addresses)
			var static_analysis_score float64
			static_analysis_score = r.CalculateNetworkRatio()
			info := &RoleInfo{num_files, num_lines, num_addresses, static_analysis_score}			
			return info, nil
		}
	}
	return nil, errors.New("Role doesn't exist")
}