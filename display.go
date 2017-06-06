package main

import (
	"fmt"
	"net"
	"strings"
)

var (
	MaxLocalAddrLength  = 17
	MaxRemoteAddrLength = 18

	NewlineFlag bool
)

func ShowSummary() {
	// Read
	if err := GenericReadSockstat(false); err != nil {
		return
	}
	if err := GenericReadSockstat(true); err != nil {
		return
	}
	// Display
	var format string
	fmt.Println("Transport\t Total\t IPv4\t IPv6\t")
	for _, protocal := range Protocal {
		if len(protocal) >= 8 {
			format = "%s\t %d\t %d\t %d\t\n"
		} else {
			format = "%s\t\t %d\t %d\t %d\t\n"
		}
		fmt.Printf(format, protocal, Summary[protocal][IPv4String]+Summary[protocal][IPv6String], Summary[protocal][IPv4String], Summary[protocal][IPv6String])
	}
}

func GenericShow(family string, records map[uint64]*GenericRecord) {
	for _, record := range records {
		if !*flagAll && !SstateActive[record.Status] {
			continue
		}
		switch family {
		case TCPv4Str, TCPv6Str:
			fmt.Printf("tcp\t")
		case UDPv4Str, UDPv6Str:
			fmt.Printf("udp\t")
		}
		if len(Sstate[record.Status]) > 8 {
			fmt.Printf("%s\t", Sstate[record.Status])
		} else {
			fmt.Printf("%s\t\t", Sstate[record.Status])
		}
		fmt.Printf("%d\t%d\t", record.RxQueue, record.TxQueue)
		fmt.Printf("%-*s\t%-*s\t", MaxLocalAddrLength, record.LocalAddr.String(), MaxRemoteAddrLength, record.RemoteAddr.String())
		// Process Info
		if *flagProcess && len(record.UserName) > 0 {
			fmt.Printf(`["%s"`, record.UserName)
			for proc := range record.Procs {
				for _, fd := range proc.Fd {
					if fd.SysStat.Ino == record.Inode {
						fmt.Printf(`(pid=%d,fd=%s)`, proc.Pid, fd.Name)
					}
				}
			}
			fmt.Printf("]")
		}
		if NewlineFlag {
			fmt.Printf("\n")
		}
		// Timer Info
		if *flagOption && record.Timer != 0 {
			fmt.Printf("[timer:(%s,%dsec,", TimerName[record.Timer], record.Timeout)
			if record.Timer != 1 {
				fmt.Printf("%d)]\t", record.Probes)
			} else {
				fmt.Printf("%d)]\t", record.Retransmit)
			}
		}
		// Detailed Info
		if *flagExtended {
			fmt.Printf("[detail:(")
			if record.UID != 0 {
				fmt.Printf("uid:%d,", record.UID)
			}
			fmt.Printf("ino:%d,sk:%x", record.Inode, record.SK)
			if len(record.Opt) > 0 {
				fmt.Printf(",opt:%v", record.Opt)
			}
			fmt.Printf(")]\t")
		}
		fmt.Printf("\n")
	}
}

func SocketShow() {
	fmt.Printf("Netid\tState\t\tRecv-Q\tSend-Q\t")
	fmt.Printf("%-*s\t%-*s\t", MaxLocalAddrLength, "LocalAddress:Port", MaxRemoteAddrLength, "RemoteAddress:Port")
	if *flagProcess {
		fmt.Printf("Users")
	}
	fmt.Printf("\n")
	if Family&FbTCPv4 != 0 {
		GenericShow(TCPv4Str, GlobalTCPv4Records)
	}
	if Family&FbTCPv6 != 0 {
		GenericShow(TCPv6Str, GlobalTCPv6Records)
	}
	if Family&FbUDPv4 != 0 {
		GenericShow(UDPv4Str, GlobalUDPv4Records)
	}
	if Family&FbUDPv6 != 0 {
		GenericShow(UDPv6Str, GlobalUDPv6Records)
	}
}

var (
	demandData = make(map[string]map[string]map[bool]map[string]bool)
	localIP    = make([]string, 0, 0)
)

func demandRecordHandler(family string, r *GenericRecord) {
	var (
		status            = Sstate[r.Status]
		procMap           map[string]map[bool]map[string]bool
		local             bool
		locOrRmtMap       map[bool]map[string]bool
		remoteRecord      *GenericRecord
		remoteServiceName string
		remoteServiceMap  map[string]bool
		ok                bool
	)
	if status != "LISTEN" && status != "ESTAB" {
		return
	}
	if len(r.UserName) == 0 {
		return
	}
	if procMap, ok = demandData[status]; !ok {
		procMap = make(map[string]map[bool]map[string]bool)
	}
	if locOrRmtMap, ok = procMap[r.UserName]; !ok {
		locOrRmtMap = make(map[bool]map[string]bool)
	}
	switch status {
	case "LISTEN":
		local = true
		if remoteServiceMap, ok = locOrRmtMap[local]; !ok {
			remoteServiceMap = make(map[string]bool)
		}
		remoteServiceMap[r.LocalAddr.String()] = true
	case "ESTAB":
		local = false
		for _, ip := range localIP {
			if strings.Contains(r.RemoteAddr.Host, ip) {
				local = true
				break
			}
		}
		if remoteServiceMap, ok = locOrRmtMap[local]; !ok {
			remoteServiceMap = make(map[string]bool)
		}
		if local {
			switch family {
			case TCPv4Str, TCPv6Str:
				for _, remoteRecord = range GlobalTCPv4Records {
					if (Sstate[remoteRecord.Status] == "LISTEN" || Sstate[remoteRecord.Status] == "ESTAB") && remoteRecord.LocalAddr.Port == r.RemoteAddr.Port {
						remoteServiceName = remoteRecord.UserName
						break
					}
				}
				for _, remoteRecord = range GlobalTCPv6Records {
					if (Sstate[remoteRecord.Status] == "LISTEN" || Sstate[remoteRecord.Status] == "ESTAB") && remoteRecord.LocalAddr.Port == r.RemoteAddr.Port {
						remoteServiceName = remoteRecord.UserName
						break
					}
				}
			case UDPv4Str, UDPv6Str:
				for _, remoteRecord = range GlobalUDPv4Records {
					if (Sstate[remoteRecord.Status] == "LISTEN" || Sstate[remoteRecord.Status] == "ESTAB") && remoteRecord.LocalAddr.Port == r.RemoteAddr.Port {
						remoteServiceName = remoteRecord.UserName
						break
					}
				}
				for _, remoteRecord = range GlobalUDPv6Records {
					if (Sstate[remoteRecord.Status] == "LISTEN" || Sstate[remoteRecord.Status] == "ESTAB") && remoteRecord.LocalAddr.Port == r.RemoteAddr.Port {
						remoteServiceName = remoteRecord.UserName
						break
					}
				}
			}
			if len(remoteServiceName) != 0 {
				remoteServiceMap[remoteServiceName] = true
			} else {
				remoteServiceMap[r.RemoteAddr.String()] = true
			}
		} else {
			remoteServiceMap[r.RemoteAddr.String()] = true
		}
	}
	locOrRmtMap[local] = remoteServiceMap
	procMap[r.UserName] = locOrRmtMap
	demandData[status] = procMap
}

func DemandShow() {
	var stringBuff []string
	localAddr, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, v := range localAddr {
		stringBuff = strings.Split(v.String(), "/")
		localIP = append(localIP, stringBuff[0])
	}
	if Family&FbTCPv4 != 0 {
		for _, record := range GlobalTCPv4Records {
			demandRecordHandler(TCPv4Str, record)
		}
	}
	if Family&FbTCPv6 != 0 {
		for _, record := range GlobalTCPv6Records {
			demandRecordHandler(TCPv6Str, record)
		}
	}
	if Family&FbUDPv4 != 0 {
		for _, record := range GlobalUDPv4Records {
			demandRecordHandler(UDPv4Str, record)
		}
	}
	if Family&FbUDPv6 != 0 {
		for _, record := range GlobalUDPv6Records {
			demandRecordHandler(UDPv6Str, record)
		}
	}
	for status, localServiceMap := range demandData {
		fmt.Println(status)
		for procName, locOrRmtMap := range localServiceMap {
			fmt.Println("\t" + procName)
			for local, remoteServiceMap := range locOrRmtMap {
				if status == "ESTAB" {
					if local {
						fmt.Println("\t\tLocal")
					} else {
						fmt.Println("\t\tRemote")
					}
				}
				for addr := range remoteServiceMap {
					fmt.Println("\t\t\t" + addr)
				}
			}
		}
	}
}
