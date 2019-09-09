package bootnode

import "testing"

func TestLocalEndpoint(t *testing.T) {
	endpoint, err := NewLocalEndpoint()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(endpoint.IP)
}
