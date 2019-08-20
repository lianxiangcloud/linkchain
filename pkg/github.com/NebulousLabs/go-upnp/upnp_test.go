package upnp

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestConcurrentUPNP tests that several threads calling Discover() concurrently
// succeed.
func TestConcurrentUPNP(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	// verify that a router exists
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := DiscoverCtx(ctx)
	if err != nil {
		t.Skip(err)
	}

	// now try to concurrently Discover() using 20 threads
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err := DiscoverCtx(ctx)
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

func TestIGD(t *testing.T) {
	// connect to router
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	d, err := DiscoverCtx(ctx)
	if err != nil {
		t.Skip(err)
	}

	// discover external IP
	ip, err := d.ExternalIP()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Your external IP is:", ip)

	// get portmap list
	for i := 0; i < 4; i++ {
		e, err := d.GetPortMappingEntry(uint16(i))
		if err != nil {
			//t.Skipf("GetPortMap %d fail: %s\n", i, err)
			continue
		}

		fmt.Printf("entry-%d: %s\n", i, e)
	}

	// forward a port
	err = d.Forward("TCP", 9001, "ipfs test")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Forward 9001")

	// record router's location
	loc := d.Location()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Loc:", loc)

	// connect to router directly
	d, err = Load(loc)
	if err != nil {
		t.Fatal(err)
	}
	intIP, _ := d.getInternalIP()
	extIP, _ := d.ExternalIP()
	t.Logf("%s --> %s", intIP, extIP)

	for i := 0; i < 4; i++ {
		e, err := d.GetPortMappingEntry(uint16(i))
		if err != nil {
			//t.Skipf("GetPortMap %d fail: %s\n", i, err)
			continue
		}

		fmt.Printf("entry-%d: %s\n", i, e)
	}

	time.Sleep(time.Second * 10)
	// un-forward a port
	err = d.Clear("TCP", 9001)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("clear 9001")
}
