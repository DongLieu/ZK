"""Hints for leveraging existing SNARK gadgets to work with Ed25519 signatures.

This file does not re-implement Ed25519 inside Python.  Instead, it documents a
concrete workflow that reuses well-tested gadgets from popular SNARK toolchains.
The goal is to help you avoid rebuilding the entire circuit from scratch.
"""

from __future__ import annotations


GUIDE = """
Recommended workflow (battle-tested in the ecosystem):

1. Circuit description
   - Pick Circom + circomlib (https://github.com/iden3/circomlib).
   - circomlib ships `eddsa.circom` which already includes:
        * Edwards point decompression/validation
        * SHA-512 hash gadget optimized for EdDSA over Baby Jubjub
        * Signature verification wiring
   - For native Ed25519, you can reuse the same EdDSA gadget provided you keep
     inputs in the Baby Jubjub subgroup.  If you truly need curve25519 arithmetic,
     use a library such as:
        * https://github.com/privacy-scaling-explorations/halo2/tree/main/halo2_gadgets
          (Ed25519 gadget in Rust/Halo2)
        * https://github.com/arkworks-rs/r1cs-std (Arkworks gadget for Ed25519)

2. Compile the circuit
   circom circuits/eddsa_verifier.circom --r1cs --wasm --sym -o build
   # Replace circuits/eddsa_verifier.circom with your wrapper that exposes
   # public inputs (message hash, public key) and private witness (signature).

3. Generate the proving/verification keys with snarkjs
   snarkjs groth16 setup build/eddsa_verifier.r1cs powersOfTau.ptau build/eddsa.zkey
   snarkjs zkey export verificationkey build/eddsa.zkey build/verification_key.json

4. Create witness
   node build/eddsa_verifier_js/generate_witness.js \
        build/eddsa_verifier_js/eddsa_verifier.wasm input.json build/witness.wtns
   # input.json carries public message/public key + private signature scalars.

5. Prove & verify
   snarkjs groth16 prove build/eddsa.zkey build/witness.wtns proof.json public.json
   snarkjs groth16 verify build/verification_key.json public.json proof.json

Python integration tip:
   - Use `subprocess.run` from this repo to call circom/snarkjs binaries.
   - Parse `public.json` for on-chain/public inputs; keep witness data off-chain.
   - If you need Go/Rust/TS bindings, export the verification key to a Solidity
     verifier or use arkworks/halo2 natively.

By leaning on these gadgets you inherit years of optimizations and audit
coverage, and only have to wire the circuit to your application-specific logic.
"""


def print_guide() -> None:
    """Print the Ed25519 SNARK integration walkthrough."""

    print(GUIDE.strip())


if __name__ == "__main__":
    print_guide()
