use bellman::groth16::{
    generate_random_parameters, create_random_proof, prepare_verifying_key, verify_proof,
};
use bellman::{Circuit, ConstraintSystem, SynthesisError};
use bls12_381::{Scalar as Fr};
use rand::thread_rng;
use bls12_381::Bls12;



// Định nghĩa mạch MultiplicationCircuit
struct MultiplicationCircuit {
    pub a: Option<Fr>,
    pub b: Option<Fr>,
    pub c: Option<Fr>,
}

impl Circuit<Fr> for MultiplicationCircuit {
    fn synthesize<CS: ConstraintSystem<Fr>>(self, cs: &mut CS) -> Result<(), SynthesisError> {
        // Biến bí mật a
        let a = cs.alloc(|| "a", || self.a.ok_or(SynthesisError::AssignmentMissing))?;
        // Biến bí mật b
        let b = cs.alloc(|| "b", || self.b.ok_or(SynthesisError::AssignmentMissing))?;
        // Biến công khai c
        let c = cs.alloc_input(|| "c", || self.c.ok_or(SynthesisError::AssignmentMissing))?;
        
        // Ràng buộc: a * b = c
        cs.enforce(
            || "multiplication constraint",
            |lc| lc + a,
            |lc| lc + b,
            |lc| lc + c,
        );
        Ok(())
    }
}

fn main() {
    let rng = &mut thread_rng();

    // Trusted setup dùng mạch mẫu (ví dụ 3*4=12)
    let circuit_setup = MultiplicationCircuit {
        a: Some(Fr::from(3u64)),
        b: Some(Fr::from(4u64)),
        c: Some(Fr::from(12u64)),
    };

    // Sinh trusted parameters cho Groth16
    let params = generate_random_parameters::<Bls12, _, _>(circuit_setup, rng).unwrap_or_else(|e| panic!("Failed to generate parameters: {:?}", e));
    let pvk = prepare_verifying_key(&params.vk);

    // Tạo 3 proof với các giá trị khác nhau
    let proof1 = {
        // Proof 1: 3 * 4 = 12
        let circuit1 = MultiplicationCircuit {
            a: Some(Fr::from(3u64)),
            b: Some(Fr::from(4u64)),
            c: Some(Fr::from(12u64)),
        };
        create_random_proof(circuit1, &params, rng).unwrap_or_else(|e| panic!("Failed to create proof1: {:?}", e))
    };

    let proof2 = {
        // Proof 2: 5 * 6 = 30
        let circuit2 = MultiplicationCircuit {
            a: Some(Fr::from(5u64)),
            b: Some(Fr::from(6u64)),
            c: Some(Fr::from(30u64)),
        };
        create_random_proof(circuit2, &params, rng).unwrap_or_else(|e| panic!("Failed to create proof1: {:?}", e))
    };

    let proof3 = {
        // Proof 3: 7 * 8 = 56
        let circuit3 = MultiplicationCircuit {
            a: Some(Fr::from(7u64)),
            b: Some(Fr::from(8u64)),
            c: Some(Fr::from(56u64)),
        };
        create_random_proof(circuit3, &params, rng).unwrap_or_else(|e| panic!("Failed to create proof1: {:?}", e))
    };

    // Public inputs (giá trị c) tương ứng
    let public_inputs1 = vec![Fr::from(12u64)];
    let public_inputs2 = vec![Fr::from(30u64)];
    let public_inputs3 = vec![Fr::from(56u64)];

    // Xác minh từng proof
    let is_valid1 = verify_proof(&pvk, &proof1, &public_inputs1)
    .unwrap_or_else(|e| panic!("Failed to verify proof1: {:?}", e));
    println!("Proof 1 valid: {:?}", is_valid1);
    let is_valid2 = verify_proof(&pvk, &proof2, &public_inputs2).unwrap_or_else(|e| panic!("Failed to create proof1: {:?}", e));
    println!("Proof 2 valid: {:?}", is_valid2);

    let is_valid3 = verify_proof(&pvk, &proof3, &public_inputs3).unwrap_or_else(|e| panic!("Failed to create proof1: {:?}", e));
    println!("Proof 3 valid: {:?}", is_valid3);
}
