package risserver

import (
	"context"
	"fmt"

	"github.com/bio-routing/bio-rd/net"
	"github.com/bio-routing/bio-rd/protocols/bgp/packet"
	"github.com/bio-routing/bio-rd/protocols/bgp/server"
	"github.com/bio-routing/bio-rd/route"
	"github.com/bio-routing/bio-rd/routingtable"
	"github.com/bio-routing/bio-rd/routingtable/filter"
	"github.com/pkg/errors"

	"github.com/prometheus/client_golang/prometheus"

	pb "github.com/bio-routing/bio-rd/cmd/multiris/api"
	bnet "github.com/bio-routing/bio-rd/net"
	netapi "github.com/bio-routing/bio-rd/net/api"
	bgpapi "github.com/bio-routing/bio-rd/protocols/bgp/api"
	routeapi "github.com/bio-routing/bio-rd/route/api"
)

var (
	risObserveFIBClients *prometheus.GaugeVec
)

const (
	localBGP = "local-bgp"
)

// RISRIB is the interface used to access a RIB by the RIS
type RISRIB interface {
	RegisterWithOptions(routingtable.RouteTableClient, routingtable.ClientOptions)
	Unregister(routingtable.RouteTableClient)
	Dump() []*route.Route
	LPM(pfx *net.Prefix) (res []*route.Route)
	Get(pfx *net.Prefix) *route.Route
	GetLonger(pfx *net.Prefix) (res []*route.Route)
}

func init() {
	risObserveFIBClients = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "bio",
			Subsystem: "ris",
			Name:      "observe_fib_clients",
			Help:      "number of observe FIB clients per router/vrf/afisafi",
		},
		[]string{
			"router",
			"vrf",
			"afisafi",
		},
	)
	prometheus.MustRegister(risObserveFIBClients)
}

// RISDataSource is something I don't know yet
type RISDataSource interface {
	hasRouter(string) bool
	getRouter() bool
}

// Server represents an RoutingInformationService server
type Server struct {
	bmp *server.BMPServer
	bgp server.BGPServer
}

// NewServer creates a new server
func NewServer(bmp *server.BMPServer, bgp server.BGPServer) *Server {
	return &Server{
		bmp: bmp,
		bgp: bgp,
	}
}

func (s Server) getRIB(rtr string, vrfID uint64, ipVersion netapi.IP_Version, neighbor string) (RISRIB, error) {
	r := s.bmp.GetRouter(rtr)

	var neighborAddr *bnet.IP
	if neighbor != "" {
		nAddr, err := bnet.IPFromString(neighbor)
		if err != nil {
			return nil, errors.Wrap(err, "Could parse neighbor address")
		}
		neighborAddr = &nAddr
	}

	if r != nil {

		v := r.GetVRF(vrfID)
		if v == nil {
			return nil, fmt.Errorf("Unable to get VRF %d", vrfID)
		}

		var rib RISRIB
		if neighborAddr == nil {
			// no neighbor, return rib of router
			switch ipVersion {
			case netapi.IP_IPv4:
				rib = v.IPv4UnicastRIB()
			case netapi.IP_IPv6:
				rib = v.IPv6UnicastRIB()
			default:
				return nil, fmt.Errorf("Unknown afi")
			}
			if rib == nil {
				return nil, fmt.Errorf("Unable to get RIB")
			}

			return rib, nil
		}

		// got a neighbor
		var ipBgpVersion uint16
		switch ipVersion {
		case netapi.IP_IPv4:
			ipBgpVersion = packet.IPv4AFI
		case netapi.IP_IPv6:
			ipBgpVersion = packet.IPv6AFI
		default:
			return nil, fmt.Errorf("Unknown afi")
		}
		rib, err := r.GetNeighborRIBIn(neighborAddr, ipBgpVersion, packet.UnicastSAFI)
		if err != nil {
			return nil, errors.Wrap(err, "Could not get neighbor RIB")
		}
		return rib, nil
	}

	//rtraddr, err := bnet.IPFromString(rtr)
	//if err == nil {
	//	peer := s.bgp.GetPeerConfig(&rtraddr)
	//	if peer != nil && peer.VRF.RD() == vrfID {
	//		var afi uint16
	//		switch ipVersion {
	//		case netapi.IP_IPv4:
	//			afi = packet.IPv4AFI
	//		case netapi.IP_IPv6:
	//			afi = packet.IPv6AFI
	//		default:
	//			return nil, fmt.Errorf("Unknown afi")
	//		}
	//		ribin := s.bgp.GetRIBIn(&rtraddr, afi, packet.UnicastSAFI)
	//		return ribin, nil
	//	}
	//}
	if rtr == localBGP {
		//switch ipVersion {
		//case netapi.IP_IPv4:
		//	afi = packet.IPv4AFI
		//case netapi.IP_IPv6:
		//	afi = packet.IPv6AFI
		//default:
		//	return nil, fmt.Errorf("Unknown afi")
		//}
		//rtraddr, err := bnet.IPFromString(rtr)
		//if err == nil {
		if neighborAddr == nil {
			// find VRF and return requested RIB
			for _, peerAddr := range s.bgp.GetPeers() {
				peer := s.bgp.GetPeerConfig(peerAddr)
				if peer != nil && peer.VRF.RD() == vrfID {
					v := peer.VRF
					switch ipVersion {
					case netapi.IP_IPv4:
						return v.IPv4UnicastRIB(), nil
					case netapi.IP_IPv6:
						return v.IPv6UnicastRIB(), nil
					default:
						return nil, fmt.Errorf("Unknown afi")
					}
				}
			}
		} else {
			peer := s.bgp.GetPeerConfig(neighborAddr)
			if peer != nil && peer.VRF.RD() == vrfID {
				var afi uint16
				switch ipVersion {
				case netapi.IP_IPv4:
					afi = packet.IPv4AFI
				case netapi.IP_IPv6:
					afi = packet.IPv6AFI
				default:
					return nil, fmt.Errorf("Unknown afi")
				}
				ribin := s.bgp.GetRIBIn(neighborAddr, afi, packet.UnicastSAFI)
				if ribin == nil {
					return nil, fmt.Errorf("RIB of BGP neighbor not yet available - peer down?")
				}
				return ribin, nil
			}
		}
		return nil, fmt.Errorf("Could not find VRF %d", vrfID)
	}

	return nil, fmt.Errorf("Unable to get router %s", rtr)
}

// LPM provides a longest prefix match service
func (s *Server) LPM(ctx context.Context, req *pb.LPMRequest) (*pb.LPMResponse, error) {
	rib, err := s.getRIB(req.Router, req.VrfId, req.Pfx.Address.Version, "")
	if err != nil {
		return nil, err
	}

	routes := rib.LPM(bnet.NewPrefixFromProtoPrefix(req.Pfx))
	res := &pb.LPMResponse{
		Routes: make([]*routeapi.Route, 0, len(routes)),
	}
	for _, route := range routes {
		res.Routes = append(res.Routes, route.ToProto())
	}

	return res, nil
}

// Get gets a prefix (exact match)
func (s *Server) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	rib, err := s.getRIB(req.Router, req.VrfId, req.Pfx.Address.Version, "")
	if err != nil {
		return nil, err
	}

	route := rib.Get(bnet.NewPrefixFromProtoPrefix(req.Pfx))
	if route == nil {
		return &pb.GetResponse{
			Routes: make([]*routeapi.Route, 0, 0),
		}, nil
	}

	return &pb.GetResponse{
		Routes: []*routeapi.Route{
			route.ToProto(),
		},
	}, nil
}

// GetLonger gets all more specifics of a prefix
func (s *Server) GetLonger(ctx context.Context, req *pb.GetLongerRequest) (*pb.GetLongerResponse, error) {
	rib, err := s.getRIB(req.Router, req.VrfId, req.Pfx.Address.Version, "")
	if err != nil {
		return nil, err
	}

	routes := rib.GetLonger(bnet.NewPrefixFromProtoPrefix(req.Pfx))
	res := &pb.GetLongerResponse{
		Routes: make([]*routeapi.Route, 0, len(routes)),
	}
	for _, route := range routes {
		res.Routes = append(res.Routes, route.ToProto())
	}

	return res, nil
}

// ObserveRIB implements the ObserveRIB RPC
func (s *Server) ObserveRIB(req *pb.ObserveRIBRequest, stream pb.RoutingInformationService_ObserveRIBServer) error {
	ipVersion := netapi.IP_IPv4
	switch req.Afisafi {
	case pb.ObserveRIBRequest_IPv4Unicast:
		ipVersion = netapi.IP_IPv4
	case pb.ObserveRIBRequest_IPv6Unicast:
		ipVersion = netapi.IP_IPv6
	default:
		return fmt.Errorf("Unknown AFI/SAFI")
	}

	rib, err := s.getRIB(req.Router, req.VrfId, ipVersion, req.Neighbor)
	if err != nil {
		return err
	}

	risObserveFIBClients.WithLabelValues(req.Router, fmt.Sprintf("%d", req.VrfId), fmt.Sprintf("%d", req.Afisafi)).Inc()
	defer risObserveFIBClients.WithLabelValues(req.Router, fmt.Sprintf("%d", req.VrfId), fmt.Sprintf("%d", req.Afisafi)).Dec()

	fifo := newUpdateFIFO()
	rc := newRIBClient(fifo)
	ret := make(chan error)

	go func(fifo *updateFIFO) {
		var err error

		for {
			for _, toSend := range fifo.dequeue() {
				err = stream.Send(toSend)
				if err != nil {
					ret <- err
					return
				}
			}

		}
	}(fifo)

	rib.RegisterWithOptions(rc, routingtable.ClientOptions{
		MaxPaths: 100,
	})
	defer rib.Unregister(rc)

	err = <-ret
	if err != nil {
		return fmt.Errorf("Stream ended: %v", err)
	}

	return nil
}

// DumpRIB implements the DumpRIB RPC
func (s *Server) DumpRIB(req *pb.DumpRIBRequest, stream pb.RoutingInformationService_DumpRIBServer) error {
	ipVersion := netapi.IP_IPv4
	switch req.Afisafi {
	case pb.DumpRIBRequest_IPv4Unicast:
		ipVersion = netapi.IP_IPv4
	case pb.DumpRIBRequest_IPv6Unicast:
		ipVersion = netapi.IP_IPv6
	default:
		return fmt.Errorf("Unknown AFI/SAFI")
	}

	rib, err := s.getRIB(req.Router, req.VrfId, ipVersion, req.Neighbor)
	if err != nil {
		return err
	}

	toSend := &pb.DumpRIBReply{
		Route: &routeapi.Route{
			Paths: make([]*routeapi.Path, 1),
		},
	}

	routes := rib.Dump()
	for i := range routes {
		toSend.Route = routes[i].ToProto()

		err = stream.Send(toSend)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetRouters implements the GetRouters RPC
func (s *Server) GetRouters(c context.Context, request *pb.GetRoutersRequest) (*pb.GetRoutersResponse, error) {
	resp := &pb.GetRoutersResponse{}
	routers := s.bmp.GetRouters()
	for _, r := range routers {
		vrfs := r.GetVRFs()
		vrfIDs := make([]uint64, 0, len(vrfs))
		for _, vrf := range vrfs {
			vrfIDs = append(vrfIDs, vrf.RD())
		}
		resp.Routers = append(resp.Routers, &pb.Router{
			SysName: r.Name(),
			VrfIds:  vrfIDs,
			Address: r.Address().String(),
			Source:  pb.Router_BMP,
		})
	}
	if s.bgp != nil {
		//for _, peerIP := range s.bgp.GetPeers() {
		//	peerCfg := s.bgp.GetPeerConfig(peerIP)
		//	vrfIDs := make([]uint64, 0, 1)
		//	vrfIDs = append(vrfIDs, peerCfg.VRF.RD())
		//	resp.Routers = append(resp.Routers, &pb.Router{
		//		SysName: peerIP.String(),
		//		VrfIds:  vrfIDs,
		//		Address: peerIP.String(),
		//	})
		//}
		resp.Routers = append(resp.Routers, &pb.Router{
			SysName: localBGP,
			VrfIds:  make([]uint64, 0),
			Address: "127.0.0.1",
			Source:  pb.Router_BGP,
		})
	}
	return resp, nil
}

// GetNeighbors for a router
func (s *Server) GetNeighbors(c context.Context, req *pb.GetNeighborsRequest) (*pb.GetNeighborsResponse, error) {
	r := s.bmp.GetRouter(req.Router)
	if r != nil {
		resp := &pb.GetNeighborsResponse{
			Neighbors: r.GetNeighborSessions(),
		}
		return resp, nil
	}

	if req.Router == localBGP && s.bgp != nil {
		bgpAPISrv := server.NewBGPAPIServer(s.bgp)
		filter := bgpapi.ListSessionsRequest{}
		neighbors, err := bgpAPISrv.ListSessions(c, &filter)
		if err != nil {
			return nil, errors.Wrap(err, "Could not get neighbors")
		}
		fmt.Printf("Neighbors got len %d with %v\n", len(neighbors.Sessions), neighbors.Sessions)
		resp := &pb.GetNeighborsResponse{
			Neighbors: neighbors.Sessions,
		}
		return resp, nil
	}

	return nil, fmt.Errorf("Could not find router")
}

type update struct {
	advertisement bool
	prefix        net.Prefix
	path          *route.Path
}

type ribClient struct {
	fifo *updateFIFO
}

func newRIBClient(fifo *updateFIFO) *ribClient {
	return &ribClient{
		fifo: fifo,
	}
}

func (r *ribClient) AddPath(pfx *net.Prefix, path *route.Path) error {
	r.fifo.queue(&pb.RIBUpdate{
		Advertisement: true,
		Route: &routeapi.Route{
			Pfx: pfx.ToProto(),
			Paths: []*routeapi.Path{
				path.ToProto(),
			},
		},
	})

	return nil
}

func (r *ribClient) RemovePath(pfx *net.Prefix, path *route.Path) bool {
	r.fifo.queue(&pb.RIBUpdate{
		Advertisement: false,
		Route: &routeapi.Route{
			Pfx: pfx.ToProto(),
			Paths: []*routeapi.Path{
				path.ToProto(),
			},
		},
	})

	return false
}

func (r *ribClient) UpdateNewClient(routingtable.RouteTableClient) error {
	return nil
}

func (r *ribClient) Register(routingtable.RouteTableClient) {
}

func (r *ribClient) RegisterWithOptions(routingtable.RouteTableClient, routingtable.ClientOptions) {
}

func (r *ribClient) Unregister(routingtable.RouteTableClient) {
}

func (r *ribClient) RouteCount() int64 {
	return -1
}

func (r *ribClient) ClientCount() uint64 {
	return 0
}

func (r *ribClient) Dump() []*route.Route {
	return nil
}

func (r *ribClient) RefreshRoute(*net.Prefix, []*route.Path) {}

func (r *ribClient) ReplaceFilterChain(filter.Chain) {}

// ReplacePath is here to fulfill an interface
func (r *ribClient) ReplacePath(*net.Prefix, *route.Path, *route.Path) {

}
