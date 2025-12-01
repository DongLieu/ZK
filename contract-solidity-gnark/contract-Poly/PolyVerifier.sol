// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @notice Minimal pairing helpers copied from the widely used snarkjs verifier layout.
library Pairing {
    uint256 internal constant SNARK_SCALAR_FIELD =
        21888242871839275222246405745257275088548364400416034343698204186575808495617;

    struct G1Point {
        uint256 X;
        uint256 Y;
    }

    struct G2Point {
        uint256[2] X;
        uint256[2] Y;
    }

    function negate(G1Point memory p) internal pure returns (G1Point memory) {
        if (p.X == 0 && p.Y == 0) {
            return G1Point(0, 0);
        }
        return G1Point(p.X, SNARK_SCALAR_FIELD - (p.Y % SNARK_SCALAR_FIELD));
    }

    function addition(G1Point memory p1, G1Point memory p2) internal view returns (G1Point memory r) {
        uint256[4] memory input;
        input[0] = p1.X;
        input[1] = p1.Y;
        input[2] = p2.X;
        input[3] = p2.Y;
        bool success;
        assembly {
            success := staticcall(gas(), 6, input, 0x80, r, 0x40)
        }
        require(success, "pairing add failed");
    }

    function scalar_mul(G1Point memory p, uint256 s) internal view returns (G1Point memory r) {
        uint256[3] memory input;
        input[0] = p.X;
        input[1] = p.Y;
        input[2] = s;
        bool success;
        assembly {
            success := staticcall(gas(), 7, input, 0x60, r, 0x40)
        }
        require(success, "pairing mul failed");
    }

    function pairing(G1Point memory p1, G2Point memory p2) internal view returns (bool) {
        G1Point[] memory p1Points = new G1Point[](1);
        G2Point[] memory p2Points = new G2Point[](1);
        p1Points[0] = p1;
        p2Points[0] = p2;
        return pairing(p1Points, p2Points);
    }

    function pairing(G1Point[] memory p1, G2Point[] memory p2) internal view returns (bool) {
        require(p1.length == p2.length, "pairing size mismatch");
        uint256 elements = p1.length;
        uint256 inputSize = elements * 6;
        uint256[] memory input = new uint256[](inputSize);
        for (uint256 i; i < elements; ++i) {
            uint256 j = i * 6;
            input[j + 0] = p1[i].X;
            input[j + 1] = p1[i].Y;
            input[j + 2] = p2[i].X[1]; // precompile expects (imag, real)
            input[j + 3] = p2[i].X[0];
            input[j + 4] = p2[i].Y[1];
            input[j + 5] = p2[i].Y[0];
        }
        uint256[1] memory out;
        bool success;
        assembly {
            success := staticcall(gas(), 8, add(input, 0x20), mul(inputSize, 0x20), out, 0x20)
        }
        require(success, "pairing op failed");
        return out[0] != 0;
    }

    function pairingProd4(
        G1Point memory a1,
        G2Point memory a2,
        G1Point memory b1,
        G2Point memory b2,
        G1Point memory c1,
        G2Point memory c2,
        G1Point memory d1,
        G2Point memory d2
    ) internal view returns (bool) {
        G1Point[] memory p1 = new G1Point[](4);
        G2Point[] memory p2 = new G2Point[](4);
        p1[0] = a1;
        p1[1] = b1;
        p1[2] = c1;
        p1[3] = d1;
        p2[0] = a2;
        p2[1] = b2;
        p2[2] = c2;
        p2[3] = d2;
        return pairing(p1, p2);
    }
}

/// @notice Verifies Groth16 proofs generated from gnark's Poly circuit (Result is the only public input).
contract PolyVerifier {
    using Pairing for *;

    uint256 private constant SNARK_SCALAR_FIELD =
        21888242871839275222246405745257275088548364400416034343698204186575808495617;

    struct VerifyingKey {
        Pairing.G1Point alpha1;
        Pairing.G2Point beta2;
        Pairing.G2Point gamma2;
        Pairing.G2Point delta2;
        Pairing.G1Point[2] ic; // len = number of public inputs + 1
    }

    struct Proof {
        Pairing.G1Point A;
        Pairing.G2Point B;
        Pairing.G1Point C;
    }

    function verifyingKey() internal pure returns (VerifyingKey memory vk) {
        vk.alpha1 = Pairing.G1Point(
            11718816689703158497786142261224236515662799412863209503833578384385425054795,
            3456962799390322794133980245791724362385020719982346688513214407521359191279
        );

        // beta, gamma, delta coordinates are already negated (matching gnark solidity export)
        vk.beta2 = Pairing.G2Point(
            [
                uint256(21674543949242153846776623425710936081062738287831304601684815362155073577398),
                uint256(12637538490493776449865246856699733160171556268911556111693443486601770267741)
            ],
            [
                uint256(14878562144931947343112612786937539481387582809832007714266093436043681472197),
                uint256(654966856686716264009850493569195186802013473471001023542171156721066577364)
            ]
        );

        vk.gamma2 = Pairing.G2Point(
            [
                uint256(3241047109053636932982530340194162364011683305086832388928291149491710249332),
                uint256(10164120541371137513160646580846369487829663635409545901501254097645019749511)
            ],
            [
                uint256(17380417886350442363619792202070117193900125705763056651401421439372364913558),
                uint256(16249244198475501009645221928231293846473350136530525730332615798821416880836)
            ]
        );

        vk.delta2 = Pairing.G2Point(
            [
                uint256(11822787107702136774239937823138083878288053153568476438761491252860197917141),
                uint256(8657006239223973298769223029261533944571817439266988988635492227155544153272)
            ],
            [
                uint256(10888769449778529932774682097083275709413699930617108434391617692247916666560),
                uint256(16830995385403411738699185605161233352092117296691160136008315430573782196119)
            ]
        );

        vk.ic[0] = Pairing.G1Point(
            17108164382990021030802319942615229348855819940729221527951406194048265151258,
            20075609252042445363714267106860938724272080356289960097975611953871588724720
        );

        vk.ic[1] = Pairing.G1Point(
            15593625996240770353237581910011869973290474518648206926895676363950321954326,
            17740577176201272729613638880123451457800768175315071386962957819296388188811
        );
    }

    function verify(uint256[] memory input, Proof memory proof) internal view returns (bool) {
        VerifyingKey memory vk = verifyingKey();
        require(input.length + 1 == vk.ic.length, "bad input length");

        Pairing.G1Point memory vkx = Pairing.G1Point(0, 0);
        vkx = Pairing.addition(vkx, vk.ic[0]);
        for (uint256 i; i < input.length; ++i) {
            require(input[i] < SNARK_SCALAR_FIELD, "input >= r");
            vkx = Pairing.addition(vkx, Pairing.scalar_mul(vk.ic[i + 1], input[i]));
        }

        return Pairing.pairingProd4(
            proof.A,
            proof.B,
            vk.alpha1,
            vk.beta2,
            vkx,
            vk.gamma2,
            proof.C,
            vk.delta2
        );
    }

    /// @notice Proof verifier that can be called directly from Remix.
    function verifyProof(
        uint256[2] calldata a,
        uint256[2][2] calldata b,
        uint256[2] calldata c,
        uint256[1] calldata input
    ) external view returns (bool) {
        Proof memory proof;
        proof.A = Pairing.G1Point(a[0], a[1]);
        proof.B = Pairing.G2Point([b[0][0], b[0][1]], [b[1][0], b[1][1]]);
        proof.C = Pairing.G1Point(c[0], c[1]);

        uint256[] memory publicInputs = new uint256[](1);
        publicInputs[0] = input[0];
        return verify(publicInputs, proof);
    }
}
