package ser

import (
	"fmt"
	"testing"
	"time"
)

func TestTime(t *testing.T) {
	/*
		// panic trying to encode times before 1970
		panicCases := []time.Time{
			time.Time{},
			time.Unix(-10, 0),
			time.Unix(0, -10),
		}
		for _, c := range panicCases {
			fmt.Printf("time sec: %d nsec: %d\n", c.Unix(), c.UnixNano())
			bz, err := EncodeToBytes(c)
			if err != nil {
				t.Errorf("Time EncodeToBytes err: %v", err)
				return
			}
			fmt.Printf("EncodeToBytes: %x\n", bz)
			var thisTime time.Time
			err = DecodeBytes(bz, &thisTime)
			fmt.Printf("this time sec: %d nsec: %d\n", thisTime.Unix(), thisTime.UnixNano())
		}
	*/
	// ensure we can encode/decode a recent time
	now := time.Now()
	fmt.Printf("time now sec: %d nsec: %d\n", now.Unix(), now.UnixNano())
	buf, err := EncodeToBytes(now)
	if err != nil {
		t.Errorf("Time EncodeToBytes err: %v", err)
		return
	}
	fmt.Printf("EncodeToBytes: %x\n", buf)

	var thisTime time.Time
	err = DecodeBytes(buf, &thisTime)
	if !thisTime.Truncate(time.Millisecond).Equal(now.Truncate(time.Millisecond)) {
		t.Errorf("times dont match. got %v, expected %v", thisTime, now)
		return
	}
	fmt.Printf("this time sec: %d nsec: %d\n", thisTime.Unix(), thisTime.UnixNano())

	nowPtr := &now
	fmt.Printf("time now sec: %d nsec: %d\n", nowPtr.Unix(), nowPtr.UnixNano())
	buf, err = EncodeToBytes(nowPtr)
	if err != nil {
		t.Errorf("Time EncodeToBytes err: %v", err)
		return
	}
	fmt.Printf("EncodeToBytes: %x\n", buf)

	var thisTime1 time.Time
	err = DecodeBytes(buf, &thisTime1)
	if !thisTime1.Truncate(time.Millisecond).Equal(now.Truncate(time.Millisecond)) {
		t.Errorf("times dont match. got %v, expected %v", thisTime, now)
		return
	}
	fmt.Printf("this time sec: %d nsec: %d\n", thisTime1.Unix(), thisTime1.UnixNano())

	type InTime struct {
		Ti time.Time
		A  int64
	}

	inTime := &InTime{
		Ti: now,
		A:  100,
	}

	buf, err = EncodeToBytes(inTime)
	if err != nil {
		t.Errorf("Time EncodeToBytes err: %v", err)
		return
	}
	fmt.Printf("EncodeToBytes: %x\n", buf)

	var inT1 InTime
	err = DecodeBytes(buf, &inT1)
	fmt.Printf("this time sec: %d nsec: %d\n", inT1.Ti.Unix(), inT1.Ti.UnixNano())

	type InTimePtr struct {
		Ti *time.Time
		A  int64
	}

	inTimePtr := &InTimePtr{
		Ti: &now,
		A:  100,
	}

	buf, err = EncodeToBytes(inTimePtr)
	if err != nil {
		t.Errorf("Time EncodeToBytes err: %v", err)
		return
	}
	fmt.Printf("EncodeToBytes: %x\n", buf)

	var inTPtr1 InTimePtr
	err = DecodeBytes(buf, &inTPtr1)
	fmt.Printf("this time sec: %d nsec: %d\n", inTPtr1.Ti.Unix(), inTPtr1.Ti.UnixNano())
}
