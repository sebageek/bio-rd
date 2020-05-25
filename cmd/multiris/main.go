package main

import (
	"flag"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"

	"github.com/bio-routing/bio-rd/cmd/multiris/config"
	"github.com/bio-routing/bio-rd/cmd/multiris/risserver"
	"github.com/bio-routing/bio-rd/protocols/bgp/server"
	"github.com/bio-routing/bio-rd/routingtable/filter"
	"github.com/bio-routing/bio-rd/routingtable/vrf"
	"github.com/bio-routing/bio-rd/util/servicewrapper"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/keepalive"

	pb "github.com/bio-routing/bio-rd/cmd/multiris/api"
	prom_bmp "github.com/bio-routing/bio-rd/metrics/bmp/adapter/prom"
	log "github.com/sirupsen/logrus"
)

var (
	grpcPort             = flag.Uint("grpc_port", 4321, "gRPC server port")
	httpPort             = flag.Uint("http_port", 4320, "HTTP server port")
	grpcKeepaliveMinTime = flag.Uint("grpc_keepalive_min_time", 1, "Minimum time (seconds) for a client to wait between GRPC keepalive pings")
	configFilePath       = flag.String("config.file", "ris_config.yml", "Configuration file")
)

func main() {
	flag.Parse()

	cfg, err := config.LoadConfig(*configFilePath)
	if err != nil {
		log.Errorf("Failed to load config: %v", err)
		os.Exit(1)
	}

	bmpSrv := server.NewServer()
	prometheus.MustRegister(prom_bmp.NewCollector(bmpSrv))
	for _, r := range cfg.BMPServers {
		ip := net.ParseIP(r.Address)
		if ip == nil {
			log.Errorf("Unable to convert %q to net.IP", r.Address)
			os.Exit(1)
		}
		bmpSrv.AddRouter(ip, r.Port)
	}

	var bgpSrv server.BGPServer = nil
	if cfg.BGPConfig.LocalASN != 0 {
		vrfReg := vrf.NewVRFRegistry()
		vrfReg.CreateVRFIfNotExists("master", 0)

		bgpSrv = server.NewBGPServer(cfg.BGPConfig.RouterIDUint32, []string{"[::]:179", "0.0.0.0:179"})

		err := bgpSrv.Start()
		if err != nil {
			log.Fatalf("Unable to start BGP server: %v", err)
			os.Exit(1)
		}

		for _, neighbor := range cfg.BGPConfig.Neighbors {
			var ttl uint8 = 1
			if neighbor.MultiHop {
				ttl = 64
			}
			peerCfg := &server.PeerConfig{
				AdminEnabled:               true,
				AdvertiseIPv4MultiProtocol: true,
				ReconnectInterval:          15 * time.Second,
				HoldTime:                   90 * time.Second,
				KeepAlive:                  30 * time.Second,
				Passive:                    true,
				RouterID:                   bgpSrv.RouterID(),
				LocalAS:                    cfg.BGPConfig.LocalASN,
				LocalAddress:               neighbor.LocalAddressIP,
				PeerAS:                     neighbor.PeerAS,
				PeerAddress:                neighbor.PeerAddressIP,
				TTL:                        ttl,
				IPv4: &server.AddressFamilyConfig{
					ImportFilterChain: filter.NewAcceptAllFilterChain(),
					ExportFilterChain: filter.NewDrainFilterChain(),
					AddPathRecv:       true,
				},
				IPv6: &server.AddressFamilyConfig{
					ImportFilterChain: filter.NewAcceptAllFilterChain(),
					ExportFilterChain: filter.NewDrainFilterChain(),
					AddPathRecv:       true,
				},
				VRF: vrfReg.GetVRFByRD(0),
			}

			bgpSrv.AddPeer(*peerCfg)
		}
	}

	s := risserver.NewServer(bmpSrv, bgpSrv)
	unaryInterceptors := []grpc.UnaryServerInterceptor{}
	streamInterceptors := []grpc.StreamServerInterceptor{}
	srv, err := servicewrapper.New(
		uint16(*grpcPort),
		servicewrapper.HTTP(uint16(*httpPort)),
		unaryInterceptors,
		streamInterceptors,
		keepalive.EnforcementPolicy{
			MinTime:             time.Duration(*grpcKeepaliveMinTime) * time.Second,
			PermitWithoutStream: true,
		},
	)
	if err != nil {
		log.Errorf("failed to listen: %v", err)
		os.Exit(1)
	}

	pb.RegisterRoutingInformationServiceServer(srv.GRPC(), s)
	if err := srv.Serve(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
