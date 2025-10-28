"""Minimal Ed25519 key generation, signing, and verification demo.

This script contains a compact, pure-Python implementation of the core
Ed25519 routines (based on RFC 8032) so that it can run without external
dependencies.  It generates a random key pair, signs a message, verifies the
signature, and demonstrates verification failure for a tampered message.
"""

from __future__ import annotations

import hashlib
import secrets
from typing import Tuple


# Field and group parameters for Ed25519.
P = 2 ** 255 - 19
L = 2 ** 252 + 27742317777372353535851937790883648493
d = (-121665 * pow(121666, P - 2, P)) % P
I = pow(2, (P - 1) // 4, P)  # sqrt(-1) mod P

Bx = 15112221349535400772501151409588531511454012693041857206046113283949847762202
By = 46316835694926478169428394003475163141307993866256225615783033603165251855960
BASE_POINT = (Bx % P, By % P)
IDENTITY = (0, 1)


def _inv(x: int) -> int:
    return pow(x, P - 2, P)


def _isoncurve(point: Tuple[int, int]) -> bool:
    x, y = point
    return (-x * x + y * y - 1 - d * x * x * y * y) % P == 0


def _edwards_add(
    point_a: Tuple[int, int], point_b: Tuple[int, int]
) -> Tuple[int, int]:
    if point_a == IDENTITY:
        return point_b
    if point_b == IDENTITY:
        return point_a

    x1, y1 = point_a
    x2, y2 = point_b
    x1x2 = (x1 * x2) % P
    y1y2 = (y1 * y2) % P
    x1y2 = (x1 * y2) % P
    y1x2 = (y1 * x2) % P

    denom_x = (1 + d * x1x2 * y1y2) % P
    denom_y = (1 - d * x1x2 * y1y2) % P
    if denom_x == 0 or denom_y == 0:
        raise ValueError("Point addition failed: denominator is zero")

    x3 = (x1y2 + y1x2) % P
    x3 = (x3 * _inv(denom_x)) % P
    y3 = (y1y2 + x1x2) % P
    y3 = (y3 * _inv(denom_y)) % P

    return x3, y3


def _scalar_mult(scalar: int, point: Tuple[int, int]) -> Tuple[int, int]:
    if scalar < 0:
        raise ValueError("Scalar multiplication expects non-negative scalars")
    result = IDENTITY
    addend = point
    k = scalar

    while k:
        if k & 1:
            result = _edwards_add(result, addend)
        addend = _edwards_add(addend, addend)
        k >>= 1

    return result


def _encode_int(number: int) -> bytes:
    return number.to_bytes(32, "little")


def _decode_int(data: bytes) -> int:
    return int.from_bytes(data, "little")


def _encode_point(point: Tuple[int, int]) -> bytes:
    x, y = point
    encoded = bytearray(_encode_int(y))
    encoded[-1] |= (x & 1) << 7
    return bytes(encoded)


def _decode_point(data: bytes) -> Tuple[int, int]:
    if len(data) != 32:
        raise ValueError("Encoded point must be 32 bytes")

    y = _decode_int(data) & ((1 << 255) - 1)
    sign = data[31] >> 7
    if y >= P:
        raise ValueError("Invalid point encoding: y out of range")

    y2 = (y * y) % P
    numerator = (y2 - 1) % P
    denominator = (d * y2 + 1) % P
    x2 = (numerator * _inv(denominator)) % P

    x = pow(x2, (P + 3) // 8, P)
    if (x * x - x2) % P != 0:
        x = (x * I) % P
    if (x * x - x2) % P != 0:
        raise ValueError("Point decompression failed")
    if x & 1 != sign:
        x = (-x) % P

    point = (x, y)
    if not _isoncurve(point):
        raise ValueError("Decoded point is not on the curve")
    return point


def _clamp_scalar(scalar_bytes: bytes) -> int:
    h = bytearray(scalar_bytes[:32])
    h[0] &= 248
    h[31] &= 63
    h[31] |= 64
    return _decode_int(bytes(h))


def _hash_mod_l(data: bytes) -> int:
    return _decode_int(hashlib.sha512(data).digest()) % L


def generate_keypair() -> Tuple[bytes, bytes]:
    secret_seed = secrets.token_bytes(32)
    hashed = hashlib.sha512(secret_seed).digest()
    a = _clamp_scalar(hashed)
    public_point = _scalar_mult(a, BASE_POINT)
    public_key = _encode_point(public_point)
    return secret_seed, public_key


def sign(message: bytes, secret_seed: bytes, public_key: bytes) -> bytes:
    hashed = hashlib.sha512(secret_seed).digest()
    a = _clamp_scalar(hashed)
    prefix = hashed[32:]

    r = _hash_mod_l(prefix + message)
    r_point = _scalar_mult(r, BASE_POINT)
    r_encoded = _encode_point(r_point)

    k = _hash_mod_l(r_encoded + public_key + message)
    s = (r + k * a) % L

    return r_encoded + _encode_int(s)


def verify(signature: bytes, message: bytes, public_key: bytes) -> bool:
    if len(signature) != 64:
        return False

    r_encoded = signature[:32]
    s = _decode_int(signature[32:])
    if s >= L:
        return False

    try:
        a_point = _decode_point(public_key)
        r_point = _decode_point(r_encoded)
    except ValueError:
        return False

    k = _hash_mod_l(r_encoded + public_key + message)
    left = _scalar_mult(s, BASE_POINT)
    right = _edwards_add(r_point, _scalar_mult(k, a_point))

    return left == right


def main() -> None:
    message = b"Xin chao, Ed25519!"
    secret_seed, public_key = generate_keypair()

    signature = sign(message, secret_seed, public_key)
    is_valid = verify(signature, message, public_key)
    tampered_valid = verify(signature, message + b"?", public_key)

    print("Secret seed :", secret_seed.hex())
    print("Public key  :", public_key.hex())
    print("Signature   :", signature.hex())
    print("Valid signature?    ", is_valid)
    print("Tampered message ok?", tampered_valid)


if __name__ == "__main__":
    main()
