package main

type Device struct {
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname,omitempty"`
	Online   bool   `json:"online"`
	NodeID   string `json:"node_id"`
}

type Devices []Device

// NodeInfo is one node in the mesh, gossiped alongside its devices.
// HTTPAddr is that node's own API address, if it exposes one -- the
// frontend uses it to let a user switch to talking to it directly.
type NodeInfo struct {
	ID       string `json:"id"`
	HTTPAddr string `json:"http_addr,omitempty"`
}

// Edge is a direct link this mesh has observed between two nodes.
type Edge struct {
	A string `json:"a"`
	B string `json:"b"`
}

type WakeRequest struct {
	MAC string `json:"mac"`
}
