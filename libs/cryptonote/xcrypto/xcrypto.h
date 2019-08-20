#ifndef _X_CRYPTO_H_
#define _X_CRYPTO_H_

#ifdef __cplusplus
extern "C" {
#endif

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>


#define X_HASH_SIZE (32)
#define X_HASH_DATA_AREA (136)

//#define int int
#define TRUE 1
#define FALSE 0




typedef char hash8_t[8];

typedef char hash_t[X_HASH_SIZE];
typedef char ec_point_t[X_HASH_SIZE];
typedef char key_image_t[X_HASH_SIZE];
typedef char ec_scalar_t[X_HASH_SIZE];
typedef char public_key_t[X_HASH_SIZE];

typedef void* p_hash8_t;
typedef void* p_hash_t;
typedef void* p_ec_point_t;
typedef void* p_key_image_t;
typedef void* p_ec_scalar_t;
typedef void* p_public_key_t;
typedef void* p_secret_key_t;

typedef struct signature {
    ec_scalar_t c;
    ec_scalar_t r;
} signature_t;


// Can contain a secret or public key
//  similar to secret_key / public_key of crypto-ops,
//  but uses unsigned chars,
//  also includes an operator for accessing the i'th byte.
typedef char rct_key_t[X_HASH_SIZE];

// typedef rct_key_t rct_key64_t[64];

typedef char* p_rct_key_t;
typedef p_rct_key_t rct_key64_t[64];
typedef p_rct_key_t* pp_rct_key_t;

//vector of keys
typedef struct rct_keyV {
    p_rct_key_t *v;
    int nums;
} rct_keyV_t;

typedef rct_keyV_t* p_rct_keyV_t;

void x_free_rct_keyV(rct_keyV_t *xx);

//matrix of keys (indexed by column first)
typedef struct rct_keyM {
    rct_keyV_t *m;
    int nums;
} rct_keyM_t;

typedef rct_keyM_t* p_rct_keyM_t;

//containers For CT operations
//if it's  representing a private ctkey then "dest" contains the secret key of the address
// while "mask" contains a where C = aG + bH is CT pedersen commitment and b is the amount
// (store b, the amount, separately
//if it's representing a public ctkey, then "dest" = P the address, mask = C the commitment
typedef struct rct_ctkey {
    p_rct_key_t dest;
    p_rct_key_t mask; //C here if public
} rct_ctkey_t;

typedef rct_ctkey_t* p_rct_ctkey_t;

typedef struct rct_ctkeyV {
    rct_ctkey_t *v;
    int nums;
} rct_ctkeyV_t;

typedef rct_ctkeyV_t* p_rct_ctkeyV_t;

void x_free_rct_ctkeyV(rct_ctkeyV_t *xx);

typedef struct rct_ctkeyM {
    rct_ctkeyV_t *m;
    int nums;
} rct_ctkeyM_t;

typedef rct_ctkeyM_t* p_rct_ctkeyM_t;


//data for passing the amount to the receiver secretly
// If the pedersen commitment to an amount is C = aG + bH,
// "mask" contains a 32 byte key a
// "amount" contains a hex representation (in 32 bytes) of a 64 bit number
// "senderPk" is not the senders actual public key, but a one-time public key generated for
// the purpose of the ECDH exchange
typedef struct rct_ecdhTuple {
    p_rct_key_t mask;
    p_rct_key_t amount;
} rct_ecdhTuple_t;
typedef rct_ecdhTuple_t* p_rct_ecdhTuple_t;

typedef struct rct_ecdhTupleV {
    rct_ecdhTuple_t *v;
    int nums;
} rct_ecdhTupleV_t;
typedef rct_ecdhTupleV_t* p_rct_ecdhTupleV_t;


typedef struct rct_sigbase {
    uint8_t Type; // Type --> type, conflict with go 
    p_rct_key_t message;
    rct_ctkeyM_t mixRing; //the set of all pubkeys / copy
    //pairs that you mix with
    rct_keyV_t pseudoOuts; //C - for simple rct
    rct_ecdhTupleV_t ecdhInfo;
    rct_ctkeyV_t outPk;
    uint64_t txnFee; // contains b

} rct_sigbase_t;

typedef struct rct_boroSig {
    rct_key64_t s0;
    rct_key64_t s1;
    p_rct_key_t ee;
} rct_boroSig_t;

//contains the data for an Borromean sig
// also contains the "Ci" values such that
// \sum Ci = C
// and the signature proves that each Ci is either
// a Pedersen commitment to 0 or to 2^i
//thus proving that C is in the range of [0, 2^64]
typedef struct rct_rangeSig {
    rct_boroSig_t asig;
    rct_key64_t Ci;
} rct_rangeSig_t;
typedef rct_rangeSig_t* p_rct_rangeSig_t;

typedef struct rct_rangeSigV {
    rct_rangeSig_t *v;
    int nums;
} rct_rangeSigV_t;
typedef rct_rangeSigV_t* p_rct_rangeSigV_t;

typedef struct rct_bulletproof {
    rct_keyV_t V;
    p_rct_key_t A, S, T1, T2;
    p_rct_key_t taux, mu;
    rct_keyV_t L, R;
    p_rct_key_t a, b, t;
} rct_bulletproof_t;
typedef rct_bulletproof_t* p_rct_bulletproof_t;

typedef struct rct_bulletproofV {
    rct_bulletproof_t *v;
    int nums;
} rct_bulletproofV_t;

//just contains the necessary keys to represent MLSAG sigs
//c.f. https://eprint.iacr.org/2015/1098
typedef struct rct_mgSig {
    rct_keyM_t ss;
    p_rct_key_t cc;
    rct_keyV_t II;
} rct_mgSig_t;
typedef rct_mgSig_t* p_rct_mgSig_t;

typedef struct rct_mgSigV {
    rct_mgSig_t *v;
    int nums;
} rct_mgSigV_t;


typedef struct rct_sig_prunable {
    rct_rangeSigV_t rangeSigs;
    rct_bulletproofV_t bulletproofs;
    rct_mgSigV_t MGs; // simple rct has N, full has 1
    rct_keyV_t pseudoOuts; //C - for simple rct
} rct_sig_prunable_t;

typedef int (*rct_sig_callback)(void*);

typedef struct rct_sig {
    rct_sigbase_t base;
    rct_sig_prunable_t p;
    rct_sig_callback cb;
    int32_t id;
} rct_sig_t;

typedef struct amountV {
    uint64_t *v;
    int nums;
} amountV_t;

typedef struct indexV {
    unsigned int *v;
    int nums;
} indexV_t;

//used for multisig data
typedef struct rct_multisig_kLRki {
    // rct_key_t k;
    // rct_key_t L;
    // rct_key_t R;
    // rct_key_t ki;

    p_rct_key_t k;
    p_rct_key_t L;
    p_rct_key_t R;
    p_rct_key_t ki;
} rct_multisig_kLRki_t;

typedef struct rct_multisig_kLRkiV {
    rct_multisig_kLRki_t *v;
    int nums;
} rct_multisig_kLRkiV_t;

typedef rct_keyV_t rct_multisig_out_t; // for all inputs

typedef struct account_public_address {
    p_public_key_t spend;
    p_public_key_t view;
} account_public_address_t;

typedef struct account_keys {
    account_public_address_t address;
    p_secret_key_t spend;
    p_secret_key_t view;
} account_keys_t;


typedef struct arr_hash {
    int     nums;
    hash_t **h;
} arr_hash_t;


//A container to hold all signatures necessary for RingCT
// rangeSigs holds all the rangeproof data of a transaction
// MG holds the MLSAG signature of a transaction
// mixRing holds all the public keypairs (P, C) for a transaction
// ecdhInfo holds an encoded mask / amount to be passed to each receiver
// outPk contains public keypairs which are destinations (P, C),
//  P = address, C = commitment to amount
enum {
    RCTTypeNull = 0,
    RCTTypeFull = 1,
    RCTTypeSimple = 2,
    RCTTypeBulletproof = 3,
    RCTTypeBulletproof2 = 4,
};

// enum RangeProofType { RangeProofBorromean, RangeProofBulletproof, RangeProofMultiOutputBulletproof, RangeProofPaddedBulletproof };

typedef struct RCTConfig {
    int range_proof_type;
    int bp_version;
} RCTConfig_t;

int x_check_ring_signature(hash_t prefix_hash, key_image_t image, rct_keyV_t *pubs, signature_t *sig);
int x_generate_ring_signature(hash_t prefix_hash, key_image_t image, rct_keyV_t *pubs, p_rct_key_t sec, size_t sec_index, signature_t *sig);

int x_cn_fast_hash(void *data, int size, hash_t result);

int x_verRctNonSemanticsSimple(rct_sig_t *rv);
int x_verRctSemanticsSimple(rct_sig_t *rv);
int x_verRctSimple(rct_sig_t *rv);
int x_verRctWithSemantics(rct_sig_t *rv, int semantics);
int x_verRct(rct_sig_t *rv);

int x_is_rct_bulletproof(int type);
void x_zeroCommit(rct_key_t ret, long long unsigned amount);
void x_slow_hash_allocate_state(void);
void x_cn_slow_hash(const void *data, int length, hash_t hash, int variant, uint64_t height);
void x_cn_slow_hash_prehashed(const void *data, int length, hash_t hash, int variant, uint64_t height);
void x_slow_hash_free_state(void);

void x_scalarmultBase(rct_key_t aG, rct_key_t a);
void x_scalarmultKey(rct_key_t aP, rct_key_t P, rct_key_t a);

void x_scalarmultH(rct_key_t aH, rct_key_t a);

void x_addKeys(rct_key_t ab, rct_key_t a,rct_key_t b);

void x_addKeys2(rct_key_t aGbB, rct_key_t a, rct_key_t b,rct_key_t B);
void x_skpkGen(rct_key_t sk, rct_key_t pk);
int x_checkKey(p_rct_key_t pk);

void x_genRct(rct_key_t message, rct_ctkeyV_t *inSk, rct_keyV_t *destinations, amountV_t *amounts, rct_ctkeyM_t *mixRing, rct_keyV_t *amount_keys,
    rct_multisig_kLRki_t *kLRki, rct_multisig_out_t *msout, unsigned int index, rct_ctkeyV_t *outSk, RCTConfig_t *rct_config, rct_sig_t *rctSig);

void x_genRctSimple(rct_key_t message, rct_ctkeyV_t *inSk, rct_keyV_t *destinations, amountV_t *inamounts, amountV_t *outamounts, uint64_t txnFee, rct_ctkeyM_t *mixRing, rct_keyV_t *amount_keys,
    rct_multisig_kLRkiV_t *kLRki, rct_multisig_out_t *msout, indexV_t *index, rct_ctkeyV_t *outSk, RCTConfig_t *rct_config, rct_sig_t *rctSig);


// void generate_random_bytes_not_thread_safe(size_t n, void *result);
int x_words_to_bytes(char *words, p_secret_key_t dst);
int x_bytes_to_words(p_secret_key_t src, char **words, char *language_name);
void x_generate_keys(p_public_key_t pub, p_secret_key_t sec, p_secret_key_t recover_key);
void x_sc_secret_add(p_secret_key_t r, p_secret_key_t a, p_secret_key_t b);
void x_get_subaddress_secret_key(p_secret_key_t sec, uint32_t index, p_secret_key_t sub_sec);
void x_get_subaddress(account_keys_t *keys, uint32_t index, account_public_address_t *pub);
void x_get_subaddress_spend_public_keys(account_keys_t *keys, uint32_t begin, uint32_t end, p_public_key_t *pubs);

int x_generate_key_derivation(p_public_key_t key1, p_secret_key_t key2, p_ec_point_t derivation);
int x_derive_subaddress_public_key(p_public_key_t pub, p_ec_point_t derivation, size_t output_index, p_public_key_t derived_pub);
int x_derive_secret_key(p_ec_point_t derivation, size_t output_index, p_secret_key_t sec, p_secret_key_t derived_sec);
int x_derive_public_key(p_ec_point_t derivation, size_t output_index, p_public_key_t pub, p_public_key_t derived_pub);
int x_secret_key_to_public_key(p_secret_key_t sec, p_public_key_t pub);
void x_hash_to_scalar(rct_keyV_t *keys, p_rct_key_t key);
int x_generate_key_image(p_public_key_t pub, p_secret_key_t sec, p_key_image_t image);
int x_derivation_to_scalar(p_ec_point_t derivation, size_t output_index, p_ec_scalar_t res);

int x_ecdh_decode(rct_ecdhTuple_t *masked, p_rct_key_t sharedSec, int short_amount);

char* x_base58_encode(char *data, int len);
char* x_base58_decode(char *addr, int *len);
char *x_base58_encode_addr(unsigned long long tag ,char *data, int len);
char *x_base58_decode_addr(unsigned long long *tag ,char *addr, int *len);

void x_scalarmult8(rct_key_t p ,rct_key_t ret);
void x_sc_add(ec_scalar_t s, ec_scalar_t a, ec_scalar_t b);
void x_sc_sub(ec_scalar_t s, ec_scalar_t a, ec_scalar_t b);
void x_skGen(rct_key_t key);
void x_genC(rct_key_t c,rct_key_t a,unsigned long long amount);
int x_ecdh_encode(rct_ecdhTuple_t *unmasked, p_rct_key_t sharedSec, int short_amount);
void x_get_pre_mlsag_hash(rct_key_t key,rct_sig_t *rv);

//tlv api
int tlv_verRctNotSemanticsSimple(unsigned char *raw, int in_len);
int tlv_verRctSimple(unsigned char *raw,int in_len);
int tlv_ecdhEncode(unsigned char *raw, int in_len, unsigned char **out);
int tlv_proveRangeBulletproof(unsigned char *raw, int in_len, unsigned char **out);
int tlv_proveRangeBulletproof128(unsigned char *raw, int in_len, unsigned char **out);
int tlv_proveRctMGSimple(rct_key_t mscout, unsigned int index,unsigned char *raw, int in_len, unsigned char **out);
int tlv_get_pre_mlsag_hash(rct_key_t key,unsigned char *raw, int in_len);
int tlv_addKeyV(rct_key_t sum, unsigned char *raw, int in_len);
int tlv_verBulletproof(unsigned char *raw, int in_len);
int tlv_verBulletproof128(unsigned char *raw, int in_len);
int tlv_get_subaddress(uint32_t index, unsigned char *raw, int in_len, unsigned char **out);

// for test
typedef void (*test_rct_key_cb)(void *);
typedef struct test_rct_key {
    p_rct_key_t data;
    test_rct_key_cb cb;
    int32_t id;
} test_rct_key_t;

typedef void (*test_rct_keyV_cb)(void *);
typedef struct test_rct_keyV {
    rct_keyV_t data;
    test_rct_keyV_cb cb;
    int32_t id;
} test_rct_keyV_t;

typedef void (*test_rct_keyM_cb)(void *);
typedef struct test_rct_keyM {
    rct_keyM_t data;
    test_rct_keyM_cb cb;
    int32_t id;
} test_rct_keyM_t;


typedef void (*test_rct_ctkey_cb)(void *);
typedef struct test_rct_ctkey {
    rct_ctkey_t data;
    test_rct_ctkey_cb cb;
    int32_t id;
} test_rct_ctkey_t;

typedef void (*test_rct_ctkeyV_cb)(void *);
typedef struct test_rct_ctkeyV {
    rct_ctkeyV_t data;
    test_rct_ctkeyV_cb cb;
    int32_t id;
} test_rct_ctkeyV_t;

typedef void (*test_rct_ctkeyM_cb)(void *);
typedef struct test_rct_ctkeyM {
    rct_ctkeyM_t data;
    test_rct_ctkeyM_cb cb;
    int32_t id;
} test_rct_ctkeyM_t;

typedef void (*test_rct_sigbase_cb)(void *);
typedef struct test_rct_sigbase {
    rct_sigbase_t data;
    test_rct_sigbase_cb cb;
    int32_t id;
} test_rct_sigbase_t;

typedef void (*test_rct_sig_prunable_cb)(void *);
typedef struct test_rct_sig_prunable {
    rct_sig_prunable_t data;
    test_rct_sig_prunable_cb cb;
    int32_t id;
} test_rct_sig_prunable_t;

typedef void (*test_rct_multisig_kLRkiV_cb)(void *);
typedef struct test_rct_multisig_kLRkiV {
    rct_multisig_kLRkiV_t data;
    test_rct_multisig_kLRkiV_cb cb;
    int32_t id;
} test_rct_multisig_kLRkiV_t;


void testc_rct_key(rct_key_t from, test_rct_key_t *to);
void testc_rct_keyV(rct_keyV_t *from, test_rct_keyV_t *to);
void testc_rct_keyM(rct_keyM_t *from, test_rct_keyM_t *to);
void testc_rct_ctkey(rct_ctkey_t *from, test_rct_ctkey_t *to);
void testc_rct_ctkeyV(rct_ctkeyV_t *from, test_rct_ctkeyV_t *to);
void testc_rct_ctkeyM(rct_ctkeyM_t *from, test_rct_ctkeyM_t *to);
void testc_rct_sigbase(rct_sigbase_t *from, test_rct_sigbase_t *to);
void testc_rct_sig_prunable(rct_sig_prunable_t *from, test_rct_sig_prunable_t *to);
void testc_rct_multisig_kLRkiV(rct_multisig_kLRkiV_t *from, test_rct_multisig_kLRkiV_t *to);
void testc_rct_sig(rct_sig_t *from, rct_sig_t *to);


int test_tlv_keyV(unsigned char *keyv_in,int keyv_in_len,unsigned char **keyv_out);

int test_tlv_rctsig(unsigned char *raw,int in_len,unsigned char **out);

#ifdef __cplusplus
}
#endif

#endif
