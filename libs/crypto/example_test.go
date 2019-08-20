package crypto

import (
	"fmt"
)

func ExampleSha256() {
	sum := Sha256([]byte("This is blockchain"))
	fmt.Printf("%x\n", sum)
	// Output:
	// 725e310f23c4ade6bbd2e042140ca3669b9a6db3d25681724ca139fcc91ca5f4
}

func ExampleRipemd160() {
	sum := Ripemd160([]byte("This is blockchain"))
	fmt.Printf("%x\n", sum)
	// Output:
	// fe3a9bddf46bc0c7a304d9e46a1f1886cebcfc64
}
