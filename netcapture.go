package main

import (
	"./compositions"
	"flag"
	"fmt"
	"log"
	"time"
)

func main() {
	node_roles_file := flag.String("roles", "./node_roles.json", "roles json file")
	config_file :=  flag.String("config", "./net_config.json", "dynamic analysis config file")
	static := flag.Bool("static", false, "Run the sattic analysis")
	dynamic := flag.Bool("dynamic", false, "Run the dynamic analysis")
	flag.Parse()
	if !(*static || * dynamic) {
		log.Fatal("Usage: go run netcapture.go [-static=true|-dynamic=true] [-roles=<role_file>] [-config=<config_file>]")
	}
	if *static {
		nr, err := compositions.InitializeNodeRoles(*node_roles_file)
		if err != nil {
			log.Fatal(err)
		}
		if nr == nil {
			log.Fatal("Failed to initialize node roles")
		}
		num_node_roles := nr.GetNumNodeRoles()
		role_names := nr.GetNodeRoleNames()
		fmt.Println("Num node roles :", num_node_roles)
		for i:=0; i < num_node_roles; i+=1 {
			fmt.Println("Info for role :", role_names[i])
			info, err := nr.GetNodeStaticAnalysisInfo(role_names[i])
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println("\t NumFiles :", info.NumFiles)
			fmt.Println("\t NumLines :", info.NumLines)
			fmt.Println("\t NumAddresses :", info.NumAddresses)
			fmt.Println("\t Score :", info.NetworkRatio)
		}
	}
	if *dynamic {
		nc, err := compositions.InitializeCapture(*config_file, 30 * time.Second)
		if err != nil {
			log.Fatal(err)
		}
		nc.StartCapture()
		time.Sleep(60 * time.Second)
		nc.StopCapture()
		log.Println(nc.Stats)
	}
}
