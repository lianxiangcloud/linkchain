package ser

import (
	"fmt"
	"testing"
)

type FStr struct {
	Name   string
	ID     int
	Salary float32
	Total  float64
}

type FStr1 struct {
	Name   string
	ID     int
	Salary *float32
	Total  *float64
}

func TestFloat(t *testing.T) {
	s := &FStr{
		Name:   "skyle",
		ID:     26,
		Salary: 168.97,
		Total:  599.34,
	}

	bz, err := EncodeToBytes(s)
	if err != nil {
		t.Errorf("EncodeToBytes err: %v", err)
	}
	fmt.Printf("EncodeToBytes bz: %x err: %v\n", bz, err)

	var s1 FStr
	err = DecodeBytes(bz, &s1)
	if err != nil {
		t.Errorf("DecodeBytes err: %v", err)
	}
	fmt.Printf("DecodeBytes ret: %v err: %v\n", s1, err)
}

func TestFloat1(t *testing.T) {
	var f1 = float32(168.97)
	var f2 = float64(599.34)
	s := &FStr1{
		Name:   "skyle",
		ID:     26,
		Salary: &f1,
		Total:  &f2,
	}

	bz, err := EncodeToBytes(s)
	if err != nil {
		t.Errorf("EncodeToBytes err: %v", err)
	}
	fmt.Printf("EncodeToBytes bz: %x err: %v\n", bz, err)

	var s1 FStr1
	err = DecodeBytes(bz, &s1)
	if err != nil {
		t.Errorf("DecodeBytes err: %v", err)
	}
	fmt.Printf("DecodeBytes ret: %v err: %v\n", s1, err)
	fmt.Printf("DecodeBytes f1: %f f2: %f err: %v\n", *s1.Salary, *s1.Total, err)
	//DecodeBytes f1: 168.970001 f2: 599.340000 err: <nil>
	fmt.Printf("DecodeBytes f1: %.2f f2: %.2f err: %v\n", *s1.Salary, *s1.Total, err)
	//DecodeBytes f1: 168.97 f2: 599.34 err: <nil>
}

func TestFloat2(t *testing.T) {
	var f1 = float32(100.00)
	bz, err := EncodeToBytes(f1)
	if err != nil {
		t.Errorf("EncodeToBytes err: %v", err)
	}
	fmt.Printf("EncodeToBytes bz: %x err: %v\n", bz, err)

	var f11 float32
	err = DecodeBytes(bz, &f11)
	if err != nil {
		t.Errorf("DecodeBytes err: %v", err)
	}
	fmt.Printf("DecodeBytes ret: %v err: %v\n", f11, err)

	var f2 = float64(299.56)
	bz, err = EncodeToBytes(f2)
	if err != nil {
		t.Errorf("EncodeToBytes err: %v", err)
	}
	fmt.Printf("EncodeToBytes bz: %x err: %v\n", bz, err)

	var f22 float64
	err = DecodeBytes(bz, &f22)
	if err != nil {
		t.Errorf("DecodeBytes err: %v", err)
	}
	fmt.Printf("DecodeBytes ret: %v err: %v\n", f22, err)

	var f3 = new(float32)
	*f3 = 1888.44
	bz, err = EncodeToBytes(f3)
	if err != nil {
		t.Errorf("EncodeToBytes err: %v", err)
	}
	fmt.Printf("EncodeToBytes bz: %x err: %v\n", bz, err)

	var f33 = new(float32)
	err = DecodeBytes(bz, &f33)
	if err != nil {
		t.Errorf("DecodeBytes err: %v", err)
	}
	fmt.Printf("DecodeBytes ret: %v err: %v\n", *f33, err)

	var f4 = new(float64)
	*f4 = 135789.88
	bz, err = EncodeToBytes(f4)
	if err != nil {
		t.Errorf("EncodeToBytes err: %v", err)
	}
	fmt.Printf("EncodeToBytes bz: %x err: %v\n", bz, err)

	var f44 = new(float64)
	err = DecodeBytes(bz, &f44)
	if err != nil {
		t.Errorf("DecodeBytes err: %v", err)
	}
	fmt.Printf("DecodeBytes ret: %v err: %v\n", *f44, err)
}
