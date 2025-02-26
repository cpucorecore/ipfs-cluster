// Package rest implements an IPFS Cluster API component. It provides
// a REST-ish API to interact with Cluster.
//
// The implented API is based on the common.API component (refer to module
// description there). The only thing this module does is to provide route
// handling for the otherwise common API component.
package rest

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/ipfs/ipfs-cluster/adder/adderutils"
	types "github.com/ipfs/ipfs-cluster/api"
	"github.com/ipfs/ipfs-cluster/api/common"
	"github.com/ipfs/ipfs-cluster/state"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
	rpc "github.com/libp2p/go-libp2p-gorpc"

	mux "github.com/gorilla/mux"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var (
	logger    = logging.Logger("restapi")
	apiLogger = logging.Logger("restapilog")
)

type peerAddBody struct {
	PeerID string `json:"peer_id"`
}

// API implements the REST API Component.
// It embeds a common.API.
type API struct {
	*common.API

	rpcClient *rpc.Client
	config    *Config
}

// NewAPI creates a new REST API component.
func NewAPI(ctx context.Context, cfg *Config) (*API, error) {
	return NewAPIWithHost(ctx, cfg, nil)
}

// NewAPI creates a new REST API component using the given libp2p Host.
func NewAPIWithHost(ctx context.Context, cfg *Config, h host.Host) (*API, error) {
	api := API{
		config: cfg,
	}
	capi, err := common.NewAPIWithHost(ctx, &cfg.Config, h, api.routes)
	api.API = capi
	return &api, err
}

// Routes returns endpoints supported by this API.
func (api *API) routes(c *rpc.Client) []common.Route {
	api.rpcClient = c
	return []common.Route{
		{
			Name:        "ID",
			Method:      "GET",
			Pattern:     "/id",
			HandlerFunc: api.idHandler,
		},

		{
			Name:        "Version",
			Method:      "GET",
			Pattern:     "/version",
			HandlerFunc: api.versionHandler,
		},

		{
			Name:        "Peers",
			Method:      "GET",
			Pattern:     "/peers",
			HandlerFunc: api.peerListHandler,
		},
		{
			Name:        "PeerAdd",
			Method:      "POST",
			Pattern:     "/peers",
			HandlerFunc: api.peerAddHandler,
		},
		{
			Name:        "PeerRemove",
			Method:      "DELETE",
			Pattern:     "/peers/{peer}",
			HandlerFunc: api.peerRemoveHandler,
		},
		{
			Name:        "Add",
			Method:      "POST",
			Pattern:     "/add",
			HandlerFunc: api.addHandler,
		},
		{
			Name:        "Allocations",
			Method:      "GET",
			Pattern:     "/allocations",
			HandlerFunc: api.allocationsHandler,
		},
		{
			Name:        "Allocation",
			Method:      "GET",
			Pattern:     "/allocations/{hash}",
			HandlerFunc: api.allocationHandler,
		},
		{
			Name:        "StatusAll",
			Method:      "GET",
			Pattern:     "/pins",
			HandlerFunc: api.statusAllHandler,
		},
		{
			Name:        "Recover",
			Method:      "POST",
			Pattern:     "/pins/{hash}/recover",
			HandlerFunc: api.recoverHandler,
		},
		{
			Name:        "RecoverAll",
			Method:      "POST",
			Pattern:     "/pins/recover",
			HandlerFunc: api.recoverAllHandler,
		},
		{
			Name:        "Status",
			Method:      "GET",
			Pattern:     "/pins/{hash}",
			HandlerFunc: api.statusHandler,
		},
		{
			Name:        "Pin",
			Method:      "POST",
			Pattern:     "/pins/{hash}",
			HandlerFunc: api.pinHandler,
		},
		{
			Name:        "PinPath",
			Method:      "POST",
			Pattern:     "/pins/{keyType:ipfs|ipns|ipld}/{path:.*}",
			HandlerFunc: api.pinPathHandler,
		},
		{
			Name:        "Unpin",
			Method:      "DELETE",
			Pattern:     "/pins/{hash}",
			HandlerFunc: api.unpinHandler,
		},
		{
			Name:        "UnpinPath",
			Method:      "DELETE",
			Pattern:     "/pins/{keyType:ipfs|ipns|ipld}/{path:.*}",
			HandlerFunc: api.unpinPathHandler,
		},
		{
			Name:        "RepoGC",
			Method:      "POST",
			Pattern:     "/ipfs/gc",
			HandlerFunc: api.repoGCHandler,
		},
		{
			Name:        "ConnectionGraph",
			Method:      "GET",
			Pattern:     "/health/graph",
			HandlerFunc: api.graphHandler,
		},
		{
			Name:        "Alerts",
			Method:      "GET",
			Pattern:     "/health/alerts",
			HandlerFunc: api.alertsHandler,
		},
		{
			Name:        "Metrics",
			Method:      "GET",
			Pattern:     "/monitor/metrics/{name}",
			HandlerFunc: api.metricsHandler,
		},
		{
			Name:        "MetricNames",
			Method:      "GET",
			Pattern:     "/monitor/metrics",
			HandlerFunc: api.metricNamesHandler,
		},
	}
}

func (api *API) idHandler(w http.ResponseWriter, r *http.Request) {
	var id types.ID
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"ID",
		struct{}{},
		&id,
	)

	api.SendResponse(w, common.SetStatusAutomatically, err, &id)
}

func (api *API) versionHandler(w http.ResponseWriter, r *http.Request) {
	var v types.Version
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"Version",
		struct{}{},
		&v,
	)

	api.SendResponse(w, common.SetStatusAutomatically, err, v)
}

func (api *API) graphHandler(w http.ResponseWriter, r *http.Request) {
	var graph types.ConnectGraph
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"ConnectGraph",
		struct{}{},
		&graph,
	)
	api.SendResponse(w, common.SetStatusAutomatically, err, graph)
}

func (api *API) metricsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	var metrics []*types.Metric
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"PeerMonitor",
		"LatestMetrics",
		name,
		&metrics,
	)
	api.SendResponse(w, common.SetStatusAutomatically, err, metrics)
}

func (api *API) metricNamesHandler(w http.ResponseWriter, r *http.Request) {
	var metricNames []string
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"PeerMonitor",
		"MetricNames",
		struct{}{},
		&metricNames,
	)
	api.SendResponse(w, common.SetStatusAutomatically, err, metricNames)
}

func (api *API) alertsHandler(w http.ResponseWriter, r *http.Request) {
	var alerts []types.Alert
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"Alerts",
		struct{}{},
		&alerts,
	)
	api.SendResponse(w, common.SetStatusAutomatically, err, alerts)
}

func (api *API) addHandler(w http.ResponseWriter, r *http.Request) {
	reader, err := r.MultipartReader()
	if err != nil {
		api.SendResponse(w, http.StatusBadRequest, err, nil)
		return
	}

	params, err := types.AddParamsFromQuery(r.URL.Query())
	if err != nil {
		api.SendResponse(w, http.StatusBadRequest, err, nil)
		return
	}

	api.SetHeaders(w)

	// any errors sent as trailer
	adderutils.AddMultipartHTTPHandler(
		r.Context(),
		api.rpcClient,
		params,
		reader,
		w,
		nil,
	)
}

func (api *API) peerListHandler(w http.ResponseWriter, r *http.Request) {
	var peers []*types.ID
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"Peers",
		struct{}{},
		&peers,
	)

	api.SendResponse(w, common.SetStatusAutomatically, err, peers)
}

func (api *API) peerAddHandler(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var addInfo peerAddBody
	err := dec.Decode(&addInfo)
	if err != nil {
		api.SendResponse(w, http.StatusBadRequest, errors.New("error decoding request body"), nil)
		return
	}

	pid, err := peer.Decode(addInfo.PeerID)
	if err != nil {
		api.SendResponse(w, http.StatusBadRequest, errors.New("error decoding peer_id"), nil)
		return
	}

	var id types.ID
	err = api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"PeerAdd",
		pid,
		&id,
	)
	api.SendResponse(w, common.SetStatusAutomatically, err, &id)
}

func (api *API) peerRemoveHandler(w http.ResponseWriter, r *http.Request) {
	if p := api.ParsePidOrFail(w, r); p != "" {
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"PeerRemove",
			p,
			&struct{}{},
		)
		api.SendResponse(w, common.SetStatusAutomatically, err, nil)
	}
}

func (api *API) pinHandler(w http.ResponseWriter, r *http.Request) {
	if pin := api.ParseCidOrFail(w, r); pin != nil {
		api.config.Logger.Debugf("rest api pinHandler: %s", pin.Cid)
		// span.AddAttributes(trace.StringAttribute("cid", pin.Cid))
		var pinObj types.Pin
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"Pin",
			pin,
			&pinObj,
		)
		api.SendResponse(w, common.SetStatusAutomatically, err, pinObj)
		api.config.Logger.Debug("rest api pinHandler done")
	}
}

func (api *API) unpinHandler(w http.ResponseWriter, r *http.Request) {
	if pin := api.ParseCidOrFail(w, r); pin != nil {
		api.config.Logger.Debugf("rest api unpinHandler: %s", pin.Cid)
		// span.AddAttributes(trace.StringAttribute("cid", pin.Cid))
		var pinObj types.Pin
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"Unpin",
			pin,
			&pinObj,
		)
		if err != nil && err.Error() == state.ErrNotFound.Error() {
			api.SendResponse(w, http.StatusNotFound, err, nil)
			return
		}
		api.SendResponse(w, common.SetStatusAutomatically, err, pinObj)
		api.config.Logger.Debug("rest api unpinHandler done")
	}
}

func (api *API) pinPathHandler(w http.ResponseWriter, r *http.Request) {
	var pin types.Pin
	if pinpath := api.ParsePinPathOrFail(w, r); pinpath != nil {
		api.config.Logger.Debugf("rest api pinPathHandler: %s", pinpath.Path)
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"PinPath",
			pinpath,
			&pin,
		)

		api.SendResponse(w, common.SetStatusAutomatically, err, pin)
		api.config.Logger.Debug("rest api pinPathHandler done")
	}
}

func (api *API) unpinPathHandler(w http.ResponseWriter, r *http.Request) {
	var pin types.Pin
	if pinpath := api.ParsePinPathOrFail(w, r); pinpath != nil {
		api.config.Logger.Debugf("rest api unpinPathHandler: %s", pinpath.Path)
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"UnpinPath",
			pinpath,
			&pin,
		)
		if err != nil && err.Error() == state.ErrNotFound.Error() {
			api.SendResponse(w, http.StatusNotFound, err, nil)
			return
		}
		api.SendResponse(w, common.SetStatusAutomatically, err, pin)
		api.config.Logger.Debug("rest api unpinPathHandler done")
	}
}

func (api *API) allocationsHandler(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()
	filterStr := queryValues.Get("filter")
	var filter types.PinType
	for _, f := range strings.Split(filterStr, ",") {
		filter |= types.PinTypeFromString(f)
	}

	if filter == types.BadType {
		api.SendResponse(w, http.StatusBadRequest, errors.New("invalid filter value"), nil)
		return
	}

	var pins []*types.Pin
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"Pins",
		struct{}{},
		&pins,
	)

	var outPins []*types.Pin

	if filter == types.AllType {
		outPins = pins
	} else {
		outPins = make([]*types.Pin, 0, len(pins))
		for _, pin := range pins {
			if filter&pin.Type > 0 {
				// add this pin to output
				outPins = append(outPins, pin)
			}
		}
	}
	api.SendResponse(w, common.SetStatusAutomatically, err, outPins)
}

func (api *API) allocationHandler(w http.ResponseWriter, r *http.Request) {
	if pin := api.ParseCidOrFail(w, r); pin != nil {
		var pinResp types.Pin
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"PinGet",
			pin.Cid,
			&pinResp,
		)
		if err != nil { // errors here are 404s
			api.SendResponse(w, http.StatusNotFound, err, nil)
			return
		}
		api.SendResponse(w, common.SetStatusAutomatically, nil, pinResp)
	}
}

func (api *API) statusAllHandler(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()
	local := queryValues.Get("local")

	var globalPinInfos []*types.GlobalPinInfo

	filterStr := queryValues.Get("filter")
	filter := types.TrackerStatusFromString(filterStr)
	if filter == types.TrackerStatusUndefined && filterStr != "" {
		api.SendResponse(w, http.StatusBadRequest, errors.New("invalid filter value"), nil)
		return
	}

	if local == "true" {
		var pinInfos []*types.PinInfo

		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"StatusAllLocal",
			filter,
			&pinInfos,
		)
		if err != nil {
			api.SendResponse(w, common.SetStatusAutomatically, err, nil)
			return
		}
		globalPinInfos = pinInfosToGlobal(pinInfos)
	} else {
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"StatusAll",
			filter,
			&globalPinInfos,
		)
		if err != nil {
			api.SendResponse(w, common.SetStatusAutomatically, err, nil)
			return
		}
	}

	api.SendResponse(w, common.SetStatusAutomatically, nil, globalPinInfos)
}

func (api *API) statusHandler(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()
	local := queryValues.Get("local")

	if pin := api.ParseCidOrFail(w, r); pin != nil {
		if local == "true" {
			var pinInfo types.PinInfo
			err := api.rpcClient.CallContext(
				r.Context(),
				"",
				"Cluster",
				"StatusLocal",
				pin.Cid,
				&pinInfo,
			)
			api.SendResponse(w, common.SetStatusAutomatically, err, pinInfo.ToGlobal())
		} else {
			var pinInfo types.GlobalPinInfo
			err := api.rpcClient.CallContext(
				r.Context(),
				"",
				"Cluster",
				"Status",
				pin.Cid,
				&pinInfo,
			)
			api.SendResponse(w, common.SetStatusAutomatically, err, pinInfo)
		}
	}
}

func (api *API) recoverAllHandler(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()
	local := queryValues.Get("local")
	if local == "true" {
		var pinInfos []*types.PinInfo
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"RecoverAllLocal",
			struct{}{},
			&pinInfos,
		)
		api.SendResponse(w, common.SetStatusAutomatically, err, pinInfosToGlobal(pinInfos))
	} else {
		var globalPinInfos []*types.GlobalPinInfo
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"RecoverAll",
			struct{}{},
			&globalPinInfos,
		)
		api.SendResponse(w, common.SetStatusAutomatically, err, globalPinInfos)
	}
}

func (api *API) recoverHandler(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()
	local := queryValues.Get("local")

	if pin := api.ParseCidOrFail(w, r); pin != nil {
		if local == "true" {
			var pinInfo types.PinInfo
			err := api.rpcClient.CallContext(
				r.Context(),
				"",
				"Cluster",
				"RecoverLocal",
				pin.Cid,
				&pinInfo,
			)
			api.SendResponse(w, common.SetStatusAutomatically, err, pinInfo.ToGlobal())
		} else {
			var pinInfo types.GlobalPinInfo
			err := api.rpcClient.CallContext(
				r.Context(),
				"",
				"Cluster",
				"Recover",
				pin.Cid,
				&pinInfo,
			)
			api.SendResponse(w, common.SetStatusAutomatically, err, pinInfo)
		}
	}
}

func (api *API) repoGCHandler(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()
	local := queryValues.Get("local")

	if local == "true" {
		var localRepoGC types.RepoGC
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"RepoGCLocal",
			struct{}{},
			&localRepoGC,
		)

		api.SendResponse(w, common.SetStatusAutomatically, err, repoGCToGlobal(&localRepoGC))
		return
	}

	var repoGC types.GlobalRepoGC
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"RepoGC",
		struct{}{},
		&repoGC,
	)
	api.SendResponse(w, common.SetStatusAutomatically, err, repoGC)
}

func repoGCToGlobal(r *types.RepoGC) types.GlobalRepoGC {
	return types.GlobalRepoGC{
		PeerMap: map[string]*types.RepoGC{
			peer.Encode(r.Peer): r,
		},
	}
}

func pinInfosToGlobal(pInfos []*types.PinInfo) []*types.GlobalPinInfo {
	gPInfos := make([]*types.GlobalPinInfo, len(pInfos))
	for i, p := range pInfos {
		gPInfos[i] = p.ToGlobal()
	}
	return gPInfos
}
