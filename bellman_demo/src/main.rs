// extern crate bellman;
// extern crate pairing;

use bellman::{Circuit, ConstraintSystem, SynthesisError};
use bls12_381::{Scalar as Fr};
// use std::str::FromStr;
use std::ops::MulAssign;

struct MyCircuit {
    pub x: Option<Fr>,
}

impl Circuit<Fr> for MyCircuit {
    fn synthesize<CS: ConstraintSystem<Fr>>(self, cs: &mut CS) -> Result<(), SynthesisError> {
        // Cấp phát biến x trong circuit
        let x = cs.alloc(|| "x", || {
            self.x.ok_or(SynthesisError::AssignmentMissing)
        })?;
        
        // Cấp phát biến x_squared cho giá trị x^2
        let x_squared = cs.alloc(|| "x_squared", || {
            let mut tmp = self.x.ok_or(SynthesisError::AssignmentMissing)?;
            tmp.mul_assign(&self.x.ok_or(SynthesisError::AssignmentMissing)?);
            Ok(tmp)
        })?;
        
        // Áp đặt ràng buộc: x * x = x_squared
        cs.enforce(
            || "square constraint",
            |lc| lc + x,
            |lc| lc + x,
            |lc| lc + x_squared,
        );
        
        Ok(())
    }
}

fn main() {
    // Khởi tạo giá trị cho x (ví dụ: x = 3)
    let x_value = Fr::from(3u64);
    
    let circuit = MyCircuit { x: Some(x_value) };
    
    println!("Đã tạo circuit thành công!");
    // Bạn có thể mở rộng để thực hiện các bước tạo chứng minh (proof) và xác minh (verification)
}
