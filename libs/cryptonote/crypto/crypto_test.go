package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
)

func TestCheckKey(t *testing.T) {
	for _, item := range getcheckKeyForTest() {
		if item.result != CheckKey(&(item.pk)) {
			t.Fatalf("TestCheckKey fail expect=%v got=%v publick=%s", item.result, !item.result, hex.EncodeToString(item.pk[:]))
		}
	}

}

func TestKeyTo(t *testing.T) {
	publickey := sToKey("c2cb3cf3840aa9893e00ec77093d3d44dba7da840b51c48462072d58d8efd183")
	key := PublicKeyToKey(publickey)
	if !bytes.Equal(publickey[:], key[:]) {
		t.Fatalf("PublicKeyToKey fail expect=%v got=%v", hex.EncodeToString(publickey[:]), hex.EncodeToString(key[:]))
	}
	publickey2 := KeyToPublicKey(key)
	if !bytes.Equal(publickey2[:], key[:]) {
		t.Fatalf("KeyToPublicKey fail expect=%v got=%v", hex.EncodeToString(key[:]), hex.EncodeToString(publickey2[:]))
	}
	secretKey := KeyToSecretKey(key)
	if !bytes.Equal(secretKey[:], key[:]) {
		t.Fatalf("KeyToPublicKey fail expect=%v got=%v", hex.EncodeToString(key[:]), hex.EncodeToString(secretKey[:]))
	}
	key1 := SecretKeyToKey(secretKey)
	if !bytes.Equal(secretKey[:], key1[:]) {
		t.Fatalf("KeyToPublicKey fail expect=%v got=%v", hex.EncodeToString(secretKey[:]), hex.EncodeToString(key1[:]))
	}

}

func TestRingSignature(t *testing.T) {
	var hash types.Hash
	var keyImage types.KeyImage
	var pubs []types.PublicKey
	var secKey types.SecretKey
	// var sig types.Signature

	b, err := hex.DecodeString("2c2516a09841352ca35aab502a33cb544dd603634419d10767b844d57f0d570f")
	if err != nil {
		panic("")
	}
	copy(hash[:], b)

	b, _ = hex.DecodeString("9797bc0f8df768f44ea13e18c0335f821a215cf971909f2e16eb3c27f79e2d1e")
	copy(keyImage[:], b)

	pubs = make([]types.PublicKey, 1)
	b, _ = hex.DecodeString("a848fa34a9eb4f3e03e195ee02dd9bd3aa29c21562d5e4cd24ed85223c32ff7b")
	copy(pubs[0][:], b)

	b, _ = hex.DecodeString("95898948acd114cb712a6e4c7a2bdd91cb9b9e3690380acf8a8b1e5f9b73960d")
	copy(secKey[:], b)

	index := uint(0)
	sig, err := GenerateRingSignature(hash, keyImage, pubs, secKey, index)
	if err != nil {
		t.Fatalf("GenerateRingSignature fail: %s", err)
	}

	if !CheckRingSignature(hash, keyImage, pubs, sig) {
		t.Fatalf("CheckRingSignature fail")
	}
}

type checkKeyResultPairForTest struct {
	pk     types.PublicKey
	result bool
}

func BenchmarkCheckKeyFalse(b *testing.B) {
	key := sToKey("c2cb3cf3840aa9893e00ec77093d3d44dba7da840b51c48462072d58d8efd183")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckKey(&key)
	}
}
func BenchmarkCheckKeyTrue(b *testing.B) {
	key := sToKey("bd85a61bae0c101d826cbed54b1290f941d26e70607a07fc6f0ad611eb8f70a6")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckKey(&key)
	}
}
func getcheckKeyForTest() []checkKeyResultPairForTest {
	return []checkKeyResultPairForTest{
		checkKeyResultPairForTest{sToKey("c2cb3cf3840aa9893e00ec77093d3d44dba7da840b51c48462072d58d8efd183"), false},
		checkKeyResultPairForTest{sToKey("bd85a61bae0c101d826cbed54b1290f941d26e70607a07fc6f0ad611eb8f70a6"), true},
		checkKeyResultPairForTest{sToKey("328f81cad4eba24ab2bad7c0e56b1e2e7346e625bcb06ae649aef3ffa0b8bef3"), false},
		checkKeyResultPairForTest{sToKey("6016a5463b9e5a58c3410d3f892b76278883473c3f0b69459172d3de49e85abe"), true},
		checkKeyResultPairForTest{sToKey("4c71282b2add07cdc6898a2622553f1ca4eb851e5cb121181628be5f3814c5b1"), false},
		checkKeyResultPairForTest{sToKey("69393c25c3b50e177f81f20f852dd604e768eb30052e23108b3cfa1a73f2736e"), true},
		checkKeyResultPairForTest{sToKey("3d5a89b676cb84c2be3428d20a660dc6a37cae13912e127888a5132e8bac2163"), true},
		checkKeyResultPairForTest{sToKey("78cd665deb28cebc6208f307734c56fccdf5fa7e2933fadfcdd2b6246e9ae95c"), false},
		checkKeyResultPairForTest{sToKey("e03b2414e260580f86ee294cd4c636a5b153e617f704e81dad248fbf715b2ee4"), true},
		checkKeyResultPairForTest{sToKey("28c3503ce82d7cdc8e0d96c4553bcf0352bbcfc73925495dbe541e7e1df105fc"), false},
		checkKeyResultPairForTest{sToKey("06855c3c3e0d03fec354059bda319b39916bdc10b6581e3f41b335ee7b014fd5"), false},
		checkKeyResultPairForTest{sToKey("556381485df0d7d5a268ab5ecfb2984b060acc63471183fcf538bf273b0c0cb5"), true},
		checkKeyResultPairForTest{sToKey("c7f76d82ac64b1e7fdc32761ff00d6f0f7ada4cf223aa5a11187e3a02e1d5319"), true},
		checkKeyResultPairForTest{sToKey("cfa85d8bdb6f633fcf031adee3a299ac42eeb6bd707744049f652f6322f5aa47"), true},
		checkKeyResultPairForTest{sToKey("91e9b63ced2b08979fee713365464cc3417c4f238f9bdd3396efbb3c58e195ee"), true},
		checkKeyResultPairForTest{sToKey("7b56e76fe94bd30b3b2f2c4ba5fe4c504821753a8965eb1cbcf8896e2d6aba19"), true},
		checkKeyResultPairForTest{sToKey("7338df494bc416cf5edcc02069e067f39cb269ce67bd9faba956021ce3b3de3a"), false},
		checkKeyResultPairForTest{sToKey("f9a1f27b1618342a558379f4815fa5039a8fe9d98a09f45c1af857ba99231dc1"), false},
		checkKeyResultPairForTest{sToKey("b2a1f37718180d4448a7fcb5f788048b1a7132dde1cfd25f0b9b01776a21c687"), true},
		checkKeyResultPairForTest{sToKey("0d3a0f9443a8b24510ad1e76a8117cca03bce416edfe35e3c2a2c2712454f8dc"), false},
		checkKeyResultPairForTest{sToKey("d8d3d806a76f120c4027dc9c9d741ad32e06861b9cfbc4ce39289c04e251bb3c"), false},
		checkKeyResultPairForTest{sToKey("1e9e3ba7bc536cd113606842835d1f05b4b9e65875742f3a35bfb2d63164b5d5"), true},
		checkKeyResultPairForTest{sToKey("5c52d0087997a2cdf1d01ed0560d94b4bfd328cb741cb9a8d46ff50374b35a57"), true},
		checkKeyResultPairForTest{sToKey("bb669d4d7ffc4b91a14defedcdbd96b330108b01adc63aa685e2165284c0033b"), false},
		checkKeyResultPairForTest{sToKey("d2709ae751a0a6fd796c98456fa95a7b64b75a3434f1caa3496eeaf5c14109b4"), true},
		checkKeyResultPairForTest{sToKey("e0c238cba781684e655b10a7d4af04ab7ff2e7022182d7ed2279d6adf36b3e7a"), false},
		checkKeyResultPairForTest{sToKey("34ebb4bf871572cee5c6935716fab8c8ec28feef4f039763d8f039b84a50bf4c"), false},
		checkKeyResultPairForTest{sToKey("4730d4f38ec3f3b83e32e6335d2506df4ee39858848842c5a0184417fcc639e4"), true},
		checkKeyResultPairForTest{sToKey("d42cf7fdf5e17e0a8a7f88505a2b7a3d297113bd93d3c20fa87e11509ec905a2"), true},
		checkKeyResultPairForTest{sToKey("b757c95059cefabb0080d3a8ebca82e46efecfd29881be3121857f9d915e388c"), false},
		checkKeyResultPairForTest{sToKey("bbe777aaf04d02b96c0632f4b1c6f35f1c7bcbc5f22af192f92c077709a2b50b"), false},
		checkKeyResultPairForTest{sToKey("73518522aabd28566f858c33fccb34b7a4de0e283f6f783f625604ee647afad9"), true},
		checkKeyResultPairForTest{sToKey("f230622c4a8f6e516590466bd10f86b64fbef61695f6a054d37604e0b024d5af"), false},
		checkKeyResultPairForTest{sToKey("bc6b9a8379fd6c369f7c3bd9ddce58db6b78f27a41d798bb865c3920824d0943"), false},
		checkKeyResultPairForTest{sToKey("45a4f87c25898cd6be105fa1602b85c4d862782adaac8b85c996c4a2bcd8af47"), true},
		checkKeyResultPairForTest{sToKey("eb4ad3561d21c4311affbd7cc2c7ff5fd509f72f88ba67dc097a75c31fdbd990"), false},
		checkKeyResultPairForTest{sToKey("2f34f4630c09a23b7ecc19f02b4190a26df69e07e13de8069ae5ff80d23762fc"), true},
		checkKeyResultPairForTest{sToKey("2ea4e4fb5085eb5c8adee0d5ab7d35c67d74d343bd816cd13924536cffc2527c"), true},
		checkKeyResultPairForTest{sToKey("5d35467ee6705a0d35818aa9ae94e4603c3e5500bfc4cf4c4f77a7160a597aa6"), true},
		checkKeyResultPairForTest{sToKey("8ff42bc76796e20c99b6e879369bd4b46a256db1366416291de9166e39d5a093"), true},
		checkKeyResultPairForTest{sToKey("0262ba718850df6c621e8a24cd9e4831c047e38818a89e15c7a06a489a4558e1"), false},
		checkKeyResultPairForTest{sToKey("58b29b2ba238b534b08fb46f05f430e61cb77dc251b0bb50afec1b6061fd9247"), false},
		checkKeyResultPairForTest{sToKey("153170e3dc2b0e1b368fc0d0e31053e872f094cdace9a2846367f0d9245a109b"), false},
		checkKeyResultPairForTest{sToKey("40419d309d07522d493bb047ca9b5fb6c401aae226eefae6fd395f5bb9114200"), true},
		checkKeyResultPairForTest{sToKey("713068818d256ef69c78cd6082492013fbd48de3c9e7e076415dd0a692994504"), true},
		checkKeyResultPairForTest{sToKey("a7218ee08e50781b0c87312d5e0031467e863c10081668e3792d96cbcee4e474"), true},
		checkKeyResultPairForTest{sToKey("356ce516b00e674ef1729c75b0a68090e7265cef675bbf32bf809495b67e9342"), false},
		checkKeyResultPairForTest{sToKey("52a5c053293675e3efd2c585047002ea6d77931cbf38f541b9070d319dc0d237"), false},
		checkKeyResultPairForTest{sToKey("77c0080bf157e069b18c4c604cc9505c5ec6f0f9930e087592d70507ca1b5534"), false},
		checkKeyResultPairForTest{sToKey("e733bc41f880a4cfb1ca6f397916504130807289cacfca10b15f5b8d058ed1bf"), false},
		checkKeyResultPairForTest{sToKey("c4f1d3c884908a574ecea8be10e02277de35ef84a1d10f105f2be996f285161f"), true},
		checkKeyResultPairForTest{sToKey("aed677f7f69e146aa0863606ac580fc0bbdc22a88c4b4386abaa4bdfff66bcc9"), false},
		checkKeyResultPairForTest{sToKey("6ad0edf59769599af8caa986f502afc67aecbebb8107aaf5e7d3ae51d5cf8dd8"), false},
		checkKeyResultPairForTest{sToKey("64a0a70e99be1f775c222ee9cd6f1bee6f632cb9417899af398ff9aff70661c6"), true},
		checkKeyResultPairForTest{sToKey("c63afaa03bb5c4ed7bc77aac175dbfb73f904440b2e3056a65850ac1bd261332"), false},
		checkKeyResultPairForTest{sToKey("a4e89cd2471c26951513b1cfbdcf053a86575e095af52495276aa56ede8ce344"), false},
		checkKeyResultPairForTest{sToKey("2ce935d97f7c3ddb973de685d20f58ee39938fe557216328045ec2b83f3132be"), true},
		checkKeyResultPairForTest{sToKey("3e3d38b1fca93c1559ac030d586616354c668aa76245a09e3fa6de55ac730973"), true},
		checkKeyResultPairForTest{sToKey("8b81b9681f76a4254007fd07ed1ded25fc675973ccb23afd06074805194733a4"), false},
		checkKeyResultPairForTest{sToKey("26d1c15dfc371489439e29bcef2afcf7ed01fac24960fdc2e7c20847a8067588"), true},
		checkKeyResultPairForTest{sToKey("85c1199b5a4591fc4cc36d23660648c1b9cfbb0e9c47199fa3eea33299a3dcec"), false},
		checkKeyResultPairForTest{sToKey("60830ba5449c1f04ac54675dfc7cac7510106c4b7549852551f8fe65971123e2"), false},
		checkKeyResultPairForTest{sToKey("3e43c28c024597b3b836e4bc16905047cbf6e841b80e0b8cd6a325049070c2a5"), false},
		checkKeyResultPairForTest{sToKey("474792c16a0032343a6f28f4cb564747c3b1ea0b6a6b9a42f7c71d7cc3dd3b44"), true},
		checkKeyResultPairForTest{sToKey("c8ec5e67cb5786673085191881950a3ca20dde88f46851b01dd91c695cfbad16"), true},
		checkKeyResultPairForTest{sToKey("861c4b24b24a87b8559e0bb665f84dcc506c147a909f335ae4573b92299f042f"), false},
		checkKeyResultPairForTest{sToKey("2c9e0fe3e4983d79f86c8c36928528f1bc90d94352ce427032cdef6906d84d0b"), true},
		checkKeyResultPairForTest{sToKey("9293742822c2dff63fdc1bf6645c864fd527cea2ddba6d4f3048d202fc340c9a"), true},
		checkKeyResultPairForTest{sToKey("3956422ad380ef19cb9fe360ef09cc7aaec7163eea4114392a7a0b2e2671914e"), true},
		checkKeyResultPairForTest{sToKey("5ae8e72cadda85e525922fec11bd53a261cf26ee230fe85a1187f831b1b2c258"), false},
		checkKeyResultPairForTest{sToKey("973feca43a0baf450c30ace5dc19015e19400f0898316e28d9f3c631da31f99a"), true},
		checkKeyResultPairForTest{sToKey("dd946c91a2077f45c5c16939e53859d9beabaf065e7b1b993d5e5cd385f8716e"), true},
		checkKeyResultPairForTest{sToKey("b3928f2d67e47f6bd6da81f72e64908d8ff391af5689f0202c4c6fec7666ffe8"), true},
		checkKeyResultPairForTest{sToKey("313382e82083697d7f9d256c3b3800b099b56c3ef33cacdccbd40a65622e25fc"), false},
		checkKeyResultPairForTest{sToKey("7d65380c12144802d39ed9306eed79fe165854273700437c0b4b50559800c058"), true},
		checkKeyResultPairForTest{sToKey("4db5c20a49422fd27739c9ca80e2271a8a125dfcead22cb8f035d0e1b7b163be"), true},
		checkKeyResultPairForTest{sToKey("dd76a9f565ef0e44d1531349ec4c5f7c3c387c2f5823e693b4952f4b0b70808c"), true},
		checkKeyResultPairForTest{sToKey("66430bf628eae23918c3ed17b42138db1f98c24819e55fc4a07452d0c85603eb"), true},
		checkKeyResultPairForTest{sToKey("9f0b677830c3f089c27daf724bb10be848537f8285de83ab0292d35afb617f77"), false},
		checkKeyResultPairForTest{sToKey("cbf98287391fb00b1e68ad64e9fb10198025864c099b8b9334d840457e673874"), true},
		checkKeyResultPairForTest{sToKey("a42552e9446e49a83aed9e3370506671216b2d1471392293b8fc2b81c81a73ee"), false},
		checkKeyResultPairForTest{sToKey("fb3de55ac81a923d506a514602d65d004ec9d13e8b47e82d73af06da73006673"), false},
		checkKeyResultPairForTest{sToKey("e17abb78e58a4b72ff4ad7387b290f2811be880b394b8bcaae7748ac09930169"), false},
		checkKeyResultPairForTest{sToKey("9ffbda7ace69753761cdb5eb01f75433efa5cdb6a4f1b664874182c6a95adcba"), true},
		checkKeyResultPairForTest{sToKey("507123c979179ea0a3f7f67fb485f71c8636ec4ec70aa47b92f3c707e7541a54"), false},
		checkKeyResultPairForTest{sToKey("f1d0b156571994ef578c61cb6545d34f834eb30e4357539a5633c862d4dffa91"), false},
		checkKeyResultPairForTest{sToKey("3de62311ec14f9ee95828c190b2dc3f03059d6119e8dfccb7323efc640e07c75"), false},
		checkKeyResultPairForTest{sToKey("5e50bb48bc9f6dd11d52c1f0d10d8ae5674d7a4af89cbbce178dafc8a562e5fe"), false},
		checkKeyResultPairForTest{sToKey("20b2c16497be101995391ceefb979814b0ea76f1ed5b6987985bcdcd17b36a81"), false},
		checkKeyResultPairForTest{sToKey("d63bff73b914ce791c840e99bfae0d47afdb99c2375e33c8f149d0df03d97873"), false},
		checkKeyResultPairForTest{sToKey("3f24b3d94b5ddd244e4c4e67a6d9f533f0396ca30454aa0ca799f21328b81d47"), true},
		checkKeyResultPairForTest{sToKey("6a44c016f09225a6d2e830290719d33eb29b53b553eea7737ed3a6e297b2e7d2"), true},
		checkKeyResultPairForTest{sToKey("ff0f34df0c76c207b8340be2009db72f730c69c2bbfeea2013105eaccf1d1f8e"), true},
		checkKeyResultPairForTest{sToKey("4baf559869fe4e915e219c3c8d9a2330fc91e542a5a2a7311d4d59fee996f807"), true},
		checkKeyResultPairForTest{sToKey("1632207dfef26e97d13b0d0035ea9468fc5a8a89b0990fce77bb143c9d7f3b67"), true},
		checkKeyResultPairForTest{sToKey("fcb3dee3993d1a47630f29410903dd03706bd5e81c5802e6f1b9095cbdb404d3"), true},
		checkKeyResultPairForTest{sToKey("fb527092b9809e3d27d7588c7ef89915a769b99c1e03e7f72bbead9ed837daae"), false},
		checkKeyResultPairForTest{sToKey("902b118d27d40ab9cbd55edd375801ce302cdb59e09c8659a3ea1401918d8bba"), false},
		checkKeyResultPairForTest{sToKey("4d6fbf25ca51e263a700f1abf84f758dde3d11b632e908b3093d64fe2e70ea0a"), true},
		checkKeyResultPairForTest{sToKey("f4c3211ec70affc1c9a94a6589460ee8360dad5f8c679152f16994038532e3fc"), true},
		checkKeyResultPairForTest{sToKey("c2b3d73ac14956d7fdf12fa92235af1bb09e1566a6a6ffd0025682c750abdd69"), false},
		checkKeyResultPairForTest{sToKey("b7e68c12207d2e2104fb2ca224829b6fccc1c0e2154e8a931e3c837a945f4430"), false},
		checkKeyResultPairForTest{sToKey("56ca0ca227708f1099bda1463db9559541c8c11ffad7b3d95c717471f25a01bf"), true},
		checkKeyResultPairForTest{sToKey("3eef3a46833e4d851671182a682e344e36bea7211a001f3b8af1093a9c83f1b2"), true},
		checkKeyResultPairForTest{sToKey("bd1f4a4f26cab7c1cbc0e17049b90854d6d28d2d55181e1b5f7a8045fcdfa06e"), true},
		checkKeyResultPairForTest{sToKey("8537b01c87e7c184d9555e8d93363dcd9b60a8acc94cd3e41eb7525fd3e1d35a"), false},
		checkKeyResultPairForTest{sToKey("68ace49179d549bad391d98ab2cc8afee65f98ce14955c3c1b16e850fabec231"), true},
		checkKeyResultPairForTest{sToKey("f9922f8a660e7c3e4f3735a817d18b72f59166a0be2d99795f953cf233a27e24"), true},
		checkKeyResultPairForTest{sToKey("036b6be3da26e80508d5a5a6a5999a1fe0db1ac4e9ade8f1ea2eaf2ea9b1a70e"), true},
		checkKeyResultPairForTest{sToKey("5e595e886ce16b5ea31f53bcb619f16c8437276618c595739fece6339731feb0"), false},
		checkKeyResultPairForTest{sToKey("4ee2cebae3476ed2eeb7efef9d20958538b3642f938403302682a04115c0f8ed"), false},
		checkKeyResultPairForTest{sToKey("519eedbd0da8676063ce7d5a605b3fc27afeecded857afa24b894ad248c87b5d"), false},
		checkKeyResultPairForTest{sToKey("ce2b627c0accf4a3105796680c37792b30c6337d2d4fea11678282455ff82ff7"), false},
		checkKeyResultPairForTest{sToKey("aa26ed99071a8416215e8e7ded784aa7c2b303aab67e66f7539905d7e922eb4d"), false},
		checkKeyResultPairForTest{sToKey("435ae49c9ca26758aa103bdcca8d51393b1906fe27a61c5245361e554f335ec2"), true},
		checkKeyResultPairForTest{sToKey("42568af395bd30024f6ccc95205c0e11a6ad1a7ee100f0ec46fcdf0af88e91fb"), false},
		checkKeyResultPairForTest{sToKey("0b4a78d1fde56181445f04ca4780f0725daa9c375b496fab6c037d6b2c2275db"), true},
		checkKeyResultPairForTest{sToKey("2f82d2a3c8ce801e1ad334f9e074a4fbf76ffac4080a7331dc1359c2b4f674a4"), false},
		checkKeyResultPairForTest{sToKey("24297d8832d733ed052dd102d4c40e813f702006f325644ccf0cb2c31f77953f"), false},
		checkKeyResultPairForTest{sToKey("5231a53f6bea7c75b273bde4a9f673044ed87796f20e0909978f29d98fc8d4f0"), true},
		checkKeyResultPairForTest{sToKey("94b5affcf78be5cf62765c32a0794bc06b4900e8a47ddba0e166ec20cec05935"), true},
		checkKeyResultPairForTest{sToKey("c14b4d846ea52ffbbb36aa62f059453af3cfae306280dada185d2d385ef8f317"), true},
		checkKeyResultPairForTest{sToKey("cceb34fddf01a6182deb79c6000a998742d4800d23d1d8472e3f43cd61f94508"), true},
		checkKeyResultPairForTest{sToKey("1faffa33407fba1634d4136cf9447896776c16293b033c6794f06774b514744c"), true},
		checkKeyResultPairForTest{sToKey("faaac98f644a2b77fb09ba0ebf5fcddf3ff55f6604c0e9e77f0278063e25113a"), true},
		checkKeyResultPairForTest{sToKey("09e8525b00bea395978279ca979247a76f38f86dce4465eb76c140a7f904c109"), true},
		checkKeyResultPairForTest{sToKey("2d797fc725e7fb6d3b412694e7386040effe4823cdf01f6ec7edea4bc0e77e20"), false},
		checkKeyResultPairForTest{sToKey("bbb74dabee651a65f46bca472df6a8a749cc4ba5ca35078df5f6d27a772f922a"), false},
		checkKeyResultPairForTest{sToKey("77513ca00f3866607c3eff5c2c011beffa775c0022c5a4e7de1120a27e6687fd"), true},
		checkKeyResultPairForTest{sToKey("10064c14ace2a998fc2843eeeb62884fe3f7ab331ca70613d6a978f44d9868eb"), false},
		checkKeyResultPairForTest{sToKey("026ae84beb5e54c62629a7b63702e85044e38cadfc9a1fcabee6099ba185005c"), false},
		checkKeyResultPairForTest{sToKey("aef91536292b7ba34a3e787fb019523c2fa7a0d56fca069cc82ccb6b02a45b14"), false},
		checkKeyResultPairForTest{sToKey("147bb1a82c623c722540feaad82b7adf4b85c6ec0cbcef3ca52906f3e85617ac"), true},
		checkKeyResultPairForTest{sToKey("fc9fb281a0847d58dc9340ef35ef02f7d20671142f12bdd1bfb324ab61d03911"), false},
		checkKeyResultPairForTest{sToKey("b739801b9455ac617ca4a7190e2806669f638d4b2f9288171afb55e1542c8d71"), false},
		checkKeyResultPairForTest{sToKey("494cc1e2ee997eb1eb051f83c4c89968116714ddf74e460d4fa1c6e7c72e3eb3"), true},
		checkKeyResultPairForTest{sToKey("ed2fbdf2b727ed9284db90ec900a942224787a880bc41d95c4bc4cf136260fd7"), true},
		checkKeyResultPairForTest{sToKey("02843d3e6fc6835ad03983670a592361a26948eb3e31648d572416a944d4909e"), true},
		checkKeyResultPairForTest{sToKey("c14fea556a7e1b6b6c3d4e2e38a4e7e95d834220ff0140d3f7f561a34e460801"), true},
		checkKeyResultPairForTest{sToKey("5f8f82a35452d0b0d09ffb40a1154641916c31e161ad1a6ab8cfddc2004efdf6"), false},
		checkKeyResultPairForTest{sToKey("7b93d72429fab07b49956007eba335bb8c5629fbf9e7a601eaa030f196934a56"), true},
		checkKeyResultPairForTest{sToKey("6a63ed96d2e46c2874beaf82344065d94b1e5c04406997f94caf4ccd97cfbab9"), false},
		checkKeyResultPairForTest{sToKey("c915f409e1e0f776d1f440aa6969cfec97559ef864b07d8c0d7c1163871b4603"), true},
		checkKeyResultPairForTest{sToKey("d06bc33630fc94303c2c369481308f805f5ce53c40141160aa4a1f072967617e"), false},
		checkKeyResultPairForTest{sToKey("1aafb14ca15043c2589bcd32c7c5f29479216a1980e127e9536729faf1c40266"), true},
		checkKeyResultPairForTest{sToKey("58c115624a20f4b0c152ccd048c54a28a938556863ab8521b154d3165d3649cd"), false},
		checkKeyResultPairForTest{sToKey("9001ba086e8aa8a67e128f36d700cc641071556306db7ec9b8ac12a6256b27b7"), false},
		checkKeyResultPairForTest{sToKey("898c468541634fb0def11f82c781341fce0def7b15695af4e642e397218c730c"), true},
		checkKeyResultPairForTest{sToKey("47ea6539e65b7b611b0e1ae9ee170adf7c31581ca9f78796d8ebbcc5cd74b712"), false},
		checkKeyResultPairForTest{sToKey("0c60952a64eeac446652f5d3c136fd36966cf66310c15ee6ab2ecbf981461257"), false},
		checkKeyResultPairForTest{sToKey("682264c4686dc7736b6e46bdc8ab231239bc5dac3f5cb9681a1e97a527945e8e"), true},
		checkKeyResultPairForTest{sToKey("276006845ca0ea4238b231434e20ad8b8b2a36876effbe1d1e3ffb1f14973397"), true},
		checkKeyResultPairForTest{sToKey("eecd3a49e55e32446f86c045dce123ef6fe2e5c57db1d850644b3c56ec689fce"), true},
		checkKeyResultPairForTest{sToKey("a4dced63589118db3d5aebf6b5670e71250f07485ca4bb6dddf9cce3e4c227a1"), false},
		checkKeyResultPairForTest{sToKey("b8ade608ba43d55db7ab481da88b74a9be513fca651c03e04d30cc79f50e0276"), false},
		checkKeyResultPairForTest{sToKey("0d91de88d007a03fe782f904808b036ff63dec6b73ce080c55231afd4ed261c3"), true},
		checkKeyResultPairForTest{sToKey("87c59becb52dd16501edadbb0e06b0406d69541c4d46115351e79951a8dd9c28"), true},
		checkKeyResultPairForTest{sToKey("9aee723be2265171fe10a86d1d3e9cf5a4e46178e859db83f86d1c6db104a247"), false},
		checkKeyResultPairForTest{sToKey("509d34ae5bf56db011845b8cdf0cc7729ed602fce765e9564cb433b4d4421a43"), false},
		checkKeyResultPairForTest{sToKey("06e766d9a6640558767c2aab29f73199130bfdc07fd858a73e6ae8e7b7ba23ba"), false},
		checkKeyResultPairForTest{sToKey("801c4fe5ab3e7cf13f7aa2ca3bc57cc8eba587d21f8bc4cd40b1e98db7aec8d9"), false},
		checkKeyResultPairForTest{sToKey("d85ad63aeb7d2faa22e5c9b87cd27f45b01e6d0fdc4c3ddf105584ac0a021465"), false},
		checkKeyResultPairForTest{sToKey("a7ca13051eb2baeb5befa5e236e482e0bb71803ad06a6eae3ae48742393329d2"), true},
		checkKeyResultPairForTest{sToKey("5a9ba3ec20f116173d933bf5cf35c320ed3751432f3ab453e4a6c51c1d243257"), false},
		checkKeyResultPairForTest{sToKey("a4091add8a6710c03285a422d6e67863a48b818f61c62e989b1e9b2ace240a87"), false},
		checkKeyResultPairForTest{sToKey("bdee0c6442e6808f25bb18e21b19032cf93a55a5f5c6426fba2227a41c748684"), true},
		checkKeyResultPairForTest{sToKey("d4aeb6cdad9667ec3b65c7fbc5bfd1b82bba1939c6bb448a86e40aec42be5f25"), false},
		checkKeyResultPairForTest{sToKey("73525b30a77f1212f7e339ec11f48c453e476f3669e6e70bebabc2fe9e37c160"), true},
		checkKeyResultPairForTest{sToKey("45501f2dc4d0a3131f9e0fe37a51c14869ab610abd8bf0158111617924953629"), false},
		checkKeyResultPairForTest{sToKey("07d0e4c592aa3676adf81cca31a95d50c8c269d995a78cde27b2a9a7a93083a6"), false},
		checkKeyResultPairForTest{sToKey("a1797d6178c18add443d22fdbf45ca5e49ead2f78b70bdf1500f570ee90adca5"), true},
		checkKeyResultPairForTest{sToKey("0961e82e6e7855d7b7bf96777e14ae729f91c5bbd20f805bd7daac5ccbec4bab"), false},
		checkKeyResultPairForTest{sToKey("57f5ba0ad36e997a4fb585cd2fc81b9cc5418db702c4d1e366639bb432d37c73"), true},
		checkKeyResultPairForTest{sToKey("82b005be61580856841e042ee8be74ae4ca66bb6733478e81ca1e56213de5c05"), false},
		checkKeyResultPairForTest{sToKey("d7733dcae1874c93e9a2bd46385f720801f913744d60479930dad7d56c767cdc"), false},
		checkKeyResultPairForTest{sToKey("b8b8b698609ac3f1bd8f4965151b43b362e6c5e3d1c1feae312c1d43976d59ab"), true},
		checkKeyResultPairForTest{sToKey("4bba7815a9a1b86a5b80b17ac0b514e2faa7a24024f269b330e5b7032ae8c04e"), true},
		checkKeyResultPairForTest{sToKey("0f70da8f8266b58acda259935ef1a947c923f8698622c5503520ff31162e877b"), false},
		checkKeyResultPairForTest{sToKey("233eaa3db80f314c6c895d1328a658a9175158fa2483ed216670c288a04b27bc"), false},
		checkKeyResultPairForTest{sToKey("a889f124fabfd7a1e2d176f485be0cbd8b3eeaafeee4f40e99e2a56befb665be"), true},
		checkKeyResultPairForTest{sToKey("2b7b8abc198b11cf7efa21bc63ec436f790fe1f9b8c044440f183ab291af61d6"), true},
		checkKeyResultPairForTest{sToKey("2491804714f7938cf501fb2adf07597b4899b919cabbaab49518b8f8767fdc6a"), true},
		checkKeyResultPairForTest{sToKey("52744a54fcb00dc930a5d7c2bc866cbfc1e75dd38b38021fd792bb0ca9f43164"), true},
		checkKeyResultPairForTest{sToKey("e42cbf70b81ba318419104dffbb0cdc3b7e7d4698e422206b753a4e2e6fc69bb"), false},
		checkKeyResultPairForTest{sToKey("2faff73e4fed62965f3dbf2e6446b5fea0364666cc8c9450b6ed63bbb6f5f0e7"), true},
		checkKeyResultPairForTest{sToKey("8b963928d75be661c3c18ddd4f4d1f37ebc095ce1edc13fe8b23784c8f416dfd"), false},
		checkKeyResultPairForTest{sToKey("b1162f952808434e4d2562ffda98bd311613d655d8cf85dc86e0a6c59f7158bc"), true},
		checkKeyResultPairForTest{sToKey("5a69adcd9e4f5b0020467e968d85877cb3aa04fa86088d4499b57ca65a665836"), true},
		checkKeyResultPairForTest{sToKey("61ab47da432c829d0bc9d4fdb59520b135428eec665ad509678188b81c7adf49"), false},
		checkKeyResultPairForTest{sToKey("154bb547f22f65a87c0c3f56294f5791d04a3c14c8125d256aeed8ec54c4a06e"), true},
		checkKeyResultPairForTest{sToKey("0a78197861c30fd3547b5f2eabd96d3ac22ac0632f03b7afd9d5d2bfc2db352f"), true},
		checkKeyResultPairForTest{sToKey("8bdeadcca1f1f8a4a67b01ed2f10ef31aba7b034e8d1df3a69fe9aebf32454e0"), false},
		checkKeyResultPairForTest{sToKey("f4b17dfca559be7d5cea500ac01e834624fed9befae3af746b39073d5f63190d"), true},
		checkKeyResultPairForTest{sToKey("622c52821e16ddc63b58f3ec2b959fe8c6ea6b1a596d9a58fd81178963f41c01"), true},
		checkKeyResultPairForTest{sToKey("07bedd5d55c937ef5e23a56c6e58f31adb91224d985285d7fef39ede3a9efb17"), false},
		checkKeyResultPairForTest{sToKey("5179bf3b7458648e57dc20f003c6bbfd55e8cd7c0a6e90df6ef8e8183b46f99d"), true},
		checkKeyResultPairForTest{sToKey("683c80c3f304f10fdd53a84813b5c25b1627ebd14eb29b258b41cd14396ef41f"), true},
		checkKeyResultPairForTest{sToKey("c266244ed597c438170875fe7874f81258a830105ca1108131e6b8fea95eb8ba"), true},
		checkKeyResultPairForTest{sToKey("0c1cdc693df29c2d1e66b2ce3747e34a30287d5eb6c302495634ec856593fe8e"), true},
		checkKeyResultPairForTest{sToKey("28950f508f6a0d4c20ab5e4d55b80565a6a539092e72b7eb0ed9fa5017ecef88"), false},
		checkKeyResultPairForTest{sToKey("8328a2a5fcfc4433b1c283539a8943e6eb8cc16c59f29dedc3af2c77cfd56f25"), true},
		checkKeyResultPairForTest{sToKey("5d0f82319676d4d3636ff5dc2a38ea5ec8aeaac4835fdcab983ab35d76b7967b"), false},
		checkKeyResultPairForTest{sToKey("cafcc75e94a014115f25c23aaae86e67352f928f468d4312b92240ff0f3a4481"), false},
		checkKeyResultPairForTest{sToKey("3e5fdd8072574218f389d018e959669e8ca4ef20b114ea7dce7bfb32339f9f42"), true},
		checkKeyResultPairForTest{sToKey("591763e3390a78ccb529ceea3d3a97165878b179ad2edaa166fd3c78ec69d391"), true},
		checkKeyResultPairForTest{sToKey("7a0a196935bf79dc2b1c3050e8f2bf0665f7773fc07511b828ec1c4b1451d317"), false},
		checkKeyResultPairForTest{sToKey("9cf0c034162131fbaa94a608f58546d0acbcc2e67b62a0b2be2ce75fc8c25b9a"), false},
		checkKeyResultPairForTest{sToKey("e3840846e3d32644d45654b96def09a5d6968caca9048c13fcaab7ae8851c316"), false},
		checkKeyResultPairForTest{sToKey("a4e330253739af588d70fbda23543f6df7d76d894a486d169e5fedf7ed32d2e2"), false},
		checkKeyResultPairForTest{sToKey("cfb41db7091223865f7ecbdda92b9a6fb08887827831451de5bcb3165395d95d"), true},
		checkKeyResultPairForTest{sToKey("3d10bd023cef8ae30229fdbfa7446a3c218423d00f330857ff6adde080749015"), false},
		checkKeyResultPairForTest{sToKey("4403b53b8d4112bb1727bb8b5fd63d1f79f107705ffe17867704e70a61875328"), false},
		checkKeyResultPairForTest{sToKey("121ef0813a9f76b7a9c045058557c5072de6a102f06a9b103ead6af079420c29"), true},
		checkKeyResultPairForTest{sToKey("386204cf473caf3854351dda55844a41162eb9ce4740e1e31cfef037b41bc56e"), false},
		checkKeyResultPairForTest{sToKey("eb5872300dc658161df469364283e4658f37f6a1349976f8973bd6b5d1d57a39"), true},
		checkKeyResultPairForTest{sToKey("b8f32188f0fc62eeb38a561ff7b7f3c94440e6d366a05ef7636958bc97834d02"), false},
		checkKeyResultPairForTest{sToKey("a817f129a8292df79eef8531736fdebb2e985304653e7ef286574d0703b40fb4"), false},
		checkKeyResultPairForTest{sToKey("2c06595bc103447b9c20a71cd358c704cb43b0b34c23fb768e6730ac9494f39e"), true},
		checkKeyResultPairForTest{sToKey("dd84bc4c366ced4f65c50c26beb8a9bc26c88b7d4a77effbb0f7af1b28e25734"), false},
		checkKeyResultPairForTest{sToKey("76b4d33810eed637f90d49a530ac5415df97cafdac6f17eda1ba7eb9a14e5886"), true},
		checkKeyResultPairForTest{sToKey("926ce5161c4c92d90ec4efc58e5f449a2c385766c42d2e60af16b7362097aef5"), false},
		checkKeyResultPairForTest{sToKey("20c661f1e95e94a745eb9ec7a4fa719eff2f64052968e448d4734f90952aefee"), false},
		checkKeyResultPairForTest{sToKey("671b50abbd119c756010416e15fcdcc9a8e92eed0f67cbca240c3a9154db55c0"), false},
		checkKeyResultPairForTest{sToKey("df7aeee8458433e5c68253b8ef006a1c74ce3aef8951056f1fa918a8eb855213"), false},
		checkKeyResultPairForTest{sToKey("70c81a38b92849cf547e3d5a6570d78e5228d4eaf9c8fdd15959edc9eb750daf"), false},
		checkKeyResultPairForTest{sToKey("55a512100b72d4ae0cfc16c75566fcaa3a7bb9116840db1559c71fd0e961cc36"), false},
		checkKeyResultPairForTest{sToKey("dbfbec4d0d2433a794ad40dc0aea965b6582875805c9a7351b47377403296acd"), true},
		checkKeyResultPairForTest{sToKey("0a7fe09eb9342214f98b38964f72ae3c787c19e5d7e256af9216f108f88b00a3"), true},
		checkKeyResultPairForTest{sToKey("a82e54681475f53ced9730ee9e3a607e341014d9403f5a42f3dbdbe8fc52e842"), true},
		checkKeyResultPairForTest{sToKey("4d1f90059f7895a3f89abf16162e8d69b399c417f515ccb43b83144bbe8105f6"), true},
		checkKeyResultPairForTest{sToKey("94e5c5b8486b1f2ff4e98ddf3b9295787eb252ba9b408ca4d7724595861da834"), false},
		checkKeyResultPairForTest{sToKey("d16e3e8dfa6d33d1d2db21c651006ccddbf4ce2e556594de5a22ae433e774ae6"), false},
		checkKeyResultPairForTest{sToKey("a1b203ec5e36098a3af08d6077068fec57eab3a754cbb5f8192983f37191c2df"), false},
		checkKeyResultPairForTest{sToKey("5378bb3ec8b4e49849bd7477356ed86f40757dd1ea3cee1e5183c7e7be4c3406"), false},
		checkKeyResultPairForTest{sToKey("541a4162edeb57130295441dc1cb604072d7323b6c7dffa02ea5e4fed1d2ee9e"), true},
		checkKeyResultPairForTest{sToKey("d8e86e189edcc4b5c262c26004691edd7bd909090997f886b00ed4b6af64d547"), false},
		checkKeyResultPairForTest{sToKey("18a8731d1983d1df2ce2703b4c85e7357b6356634ac1412e6c2ac33ad35f8364"), false},
		checkKeyResultPairForTest{sToKey("b21212eac1eb11e811022514c5041233c4a07083a5b20acd7d632a938dc627de"), true},
		checkKeyResultPairForTest{sToKey("50efcfac1a55e9829d89334513d6d921abeb237594174015d154512054e4f9d1"), true},
		checkKeyResultPairForTest{sToKey("9c44e8bcba31ddb4e67808422e42062540742ebd73439da0ba7837bf26649ec4"), true},
		checkKeyResultPairForTest{sToKey("b068a4f90d5bd78fd350daa129de35e5297b0ad6be9c85c7a6f129e3760a1482"), false},
		checkKeyResultPairForTest{sToKey("e9df93932f0096fcf2055564457c6dc685051673a4a6cd87779924be5c4abead"), true},
		checkKeyResultPairForTest{sToKey("eddab2fc52dac8ed12914d1eb5b0da9978662c4d35b388d64ddf8f065606acaf"), true},
		checkKeyResultPairForTest{sToKey("54d3e6b3f2143d9083b4c98e4c22d98f99d274228050b2dc11695bf86631e89f"), true},
		checkKeyResultPairForTest{sToKey("6da1d5ef1827de8bbf886623561b058032e196d17f983cbc52199b31b2acc75b"), true},
		checkKeyResultPairForTest{sToKey("e2a2df18e2235ebd743c9714e334f415d4ca4baf7ad1b335fb45021353d5117f"), true},
		checkKeyResultPairForTest{sToKey("f34cb7d6e861c8bfe6e15ac19de68e74ccc9b345a7b751a10a5c7f85a99dfeb6"), false},
		checkKeyResultPairForTest{sToKey("f36e2f5967eb56244f9e4981a831f4d19c805e31983662641fe384e68176604a"), true},
		checkKeyResultPairForTest{sToKey("c7e2dc9e8aa6f9c23d379e0f5e3057a69b931b886bbb74ded9f660c06d457463"), true},
		checkKeyResultPairForTest{sToKey("b97324364941e06f2ab4f5153a368f9b07c524a89e246720099042ad9e8c1c5b"), false},
		checkKeyResultPairForTest{sToKey("eff75c70d425f5bba0eef426e116a4697e54feefac870660d9cf24c685078d75"), false},
		checkKeyResultPairForTest{sToKey("161f3cd1a5873788755437e399136bcbf51ff5534700b3a8064f822995a15d24"), false},
		checkKeyResultPairForTest{sToKey("63d6d3d2c21e88b06c9ff856809572024d86c85d85d6d62a52105c0672d92e66"), false},
		checkKeyResultPairForTest{sToKey("1dc19b610b293de602f43dca6c204ce304702e6dc15d2a9337da55961bd26834"), false},
		checkKeyResultPairForTest{sToKey("28a16d02405f509e1cfef5236c0c5f73c3bcadcd23c8eff377253941f82769db"), true},
		checkKeyResultPairForTest{sToKey("682d9cc3b65d149b8c2e54d6e20101e12b7cf96be90c9458e7a69699ec0c8ed7"), false},
		checkKeyResultPairForTest{sToKey("0000000000000000000000000000000000000000000000000000000000000000"), true},
		checkKeyResultPairForTest{sToKey("0000000000000000000000000000000000000000000000000000000000000080"), true},
		checkKeyResultPairForTest{sToKey("0100000000000000000000000000000000000000000000000000000000000000"), true},
		checkKeyResultPairForTest{sToKey("0100000000000000000000000000000000000000000000000000000000000080"), false},
		checkKeyResultPairForTest{sToKey("0200000000000000000000000000000000000000000000000000000000000000"), false},
		checkKeyResultPairForTest{sToKey("0200000000000000000000000000000000000000000000000000000000000080"), false},
		checkKeyResultPairForTest{sToKey("0300000000000000000000000000000000000000000000000000000000000000"), true},
		checkKeyResultPairForTest{sToKey("0300000000000000000000000000000000000000000000000000000000000080"), true},
		checkKeyResultPairForTest{sToKey("0400000000000000000000000000000000000000000000000000000000000000"), true},
		checkKeyResultPairForTest{sToKey("0400000000000000000000000000000000000000000000000000000000000080"), true},
		checkKeyResultPairForTest{sToKey("0500000000000000000000000000000000000000000000000000000000000000"), true},
		checkKeyResultPairForTest{sToKey("0500000000000000000000000000000000000000000000000000000000000080"), true},
		checkKeyResultPairForTest{sToKey("0600000000000000000000000000000000000000000000000000000000000000"), true},
		checkKeyResultPairForTest{sToKey("0600000000000000000000000000000000000000000000000000000000000080"), true},
		checkKeyResultPairForTest{sToKey("0700000000000000000000000000000000000000000000000000000000000000"), false},
		checkKeyResultPairForTest{sToKey("0700000000000000000000000000000000000000000000000000000000000080"), false},
		checkKeyResultPairForTest{sToKey("0800000000000000000000000000000000000000000000000000000000000000"), false},
		checkKeyResultPairForTest{sToKey("0800000000000000000000000000000000000000000000000000000000000080"), false},
		checkKeyResultPairForTest{sToKey("0900000000000000000000000000000000000000000000000000000000000000"), true},
		checkKeyResultPairForTest{sToKey("0900000000000000000000000000000000000000000000000000000000000080"), true},
		checkKeyResultPairForTest{sToKey("0a00000000000000000000000000000000000000000000000000000000000000"), true},
		checkKeyResultPairForTest{sToKey("0a00000000000000000000000000000000000000000000000000000000000080"), true},
		checkKeyResultPairForTest{sToKey("0b00000000000000000000000000000000000000000000000000000000000000"), false},
		checkKeyResultPairForTest{sToKey("0b00000000000000000000000000000000000000000000000000000000000080"), false},
		checkKeyResultPairForTest{sToKey("0c00000000000000000000000000000000000000000000000000000000000000"), false},
		checkKeyResultPairForTest{sToKey("0c00000000000000000000000000000000000000000000000000000000000080"), false},
		checkKeyResultPairForTest{sToKey("0d00000000000000000000000000000000000000000000000000000000000000"), false},
		checkKeyResultPairForTest{sToKey("0d00000000000000000000000000000000000000000000000000000000000080"), false},
		checkKeyResultPairForTest{sToKey("0e00000000000000000000000000000000000000000000000000000000000000"), true},
		checkKeyResultPairForTest{sToKey("0e00000000000000000000000000000000000000000000000000000000000080"), true},
		checkKeyResultPairForTest{sToKey("0f00000000000000000000000000000000000000000000000000000000000000"), true},
		checkKeyResultPairForTest{sToKey("0f00000000000000000000000000000000000000000000000000000000000080"), true},
		checkKeyResultPairForTest{sToKey("1000000000000000000000000000000000000000000000000000000000000000"), true},
		checkKeyResultPairForTest{sToKey("1000000000000000000000000000000000000000000000000000000000000080"), true},
		checkKeyResultPairForTest{sToKey("1100000000000000000000000000000000000000000000000000000000000000"), false},
		checkKeyResultPairForTest{sToKey("1100000000000000000000000000000000000000000000000000000000000080"), false},
		checkKeyResultPairForTest{sToKey("1200000000000000000000000000000000000000000000000000000000000000"), true},
		checkKeyResultPairForTest{sToKey("1200000000000000000000000000000000000000000000000000000000000080"), true},
		checkKeyResultPairForTest{sToKey("1300000000000000000000000000000000000000000000000000000000000000"), true},
		checkKeyResultPairForTest{sToKey("1300000000000000000000000000000000000000000000000000000000000080"), true},
		checkKeyResultPairForTest{sToKey("daffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), true},
		checkKeyResultPairForTest{sToKey("daffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), true},
		checkKeyResultPairForTest{sToKey("dbffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), true},
		checkKeyResultPairForTest{sToKey("dbffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), true},
		checkKeyResultPairForTest{sToKey("dcffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("dcffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("ddffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), true},
		checkKeyResultPairForTest{sToKey("ddffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), true},
		checkKeyResultPairForTest{sToKey("deffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), true},
		checkKeyResultPairForTest{sToKey("deffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), true},
		checkKeyResultPairForTest{sToKey("dfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), true},
		checkKeyResultPairForTest{sToKey("dfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), true},
		checkKeyResultPairForTest{sToKey("e0ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("e0ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("e1ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("e1ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("e2ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("e2ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("e3ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), true},
		checkKeyResultPairForTest{sToKey("e3ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), true},
		checkKeyResultPairForTest{sToKey("e4ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), true},
		checkKeyResultPairForTest{sToKey("e4ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), true},
		checkKeyResultPairForTest{sToKey("e5ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("e5ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("e6ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("e6ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("e7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), true},
		checkKeyResultPairForTest{sToKey("e7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), true},
		checkKeyResultPairForTest{sToKey("e8ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), true},
		checkKeyResultPairForTest{sToKey("e8ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), true},
		checkKeyResultPairForTest{sToKey("e9ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), true},
		checkKeyResultPairForTest{sToKey("e9ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), true},
		checkKeyResultPairForTest{sToKey("eaffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), true},
		checkKeyResultPairForTest{sToKey("eaffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), true},
		checkKeyResultPairForTest{sToKey("ebffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("ebffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), true},
		checkKeyResultPairForTest{sToKey("ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("efffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("efffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("f0ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("f0ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("f1ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("f1ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("f2ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("f2ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("f3ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("f3ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("f4ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("f4ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("f5ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("f5ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("f6ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("f6ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("f7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("f7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("f8ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("f8ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("f9ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("f9ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("faffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("faffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("fbffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("fbffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("fcffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("fcffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("fdffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("fdffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("feffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("feffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
		checkKeyResultPairForTest{sToKey("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f"), false},
		checkKeyResultPairForTest{sToKey("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), false},
	}
}
func sToKey(s string) (pk types.PublicKey) {
	x, err := hex.DecodeString(s)
	if err != nil {
		panic(err.Error())
	}
	copy(pk[:], x[:])
	return pk
}
