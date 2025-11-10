use cosmwasm_std::{
    entry_point, to_json_binary, Addr, Binary, Deps, DepsMut, Env, MessageInfo, Response, StdError,
    StdResult, Uint128,
};
use cw_storage_plus::Item;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use sp1_verifier::{Groth16Error, Groth16Verifier};
use thiserror::Error;

const CONFIG: Item<Config> = Item::new("config");
const FIB_MODULUS: u32 = 7919;

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct InstantiateMsg {
    /// Optional upper bound on the Fibonacci index that can be verified.
    pub max_n: Option<u64>,
    /// SP1 Groth16 verifying key bytes (compressed, BN254 curve).
    pub verifying_key: Binary,
    /// SP1 program verification key hash (`vk.bytes32()`).
    pub sp1_vkey_hash: String,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum ExecuteMsg {
    Verify {
        n: u64,
        value: Uint128,
        proof: Binary,
    },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum QueryMsg {
    Expected { n: u64 },
    Config {},
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct ExpectedResponse {
    pub value: Uint128,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct ConfigResponse {
    pub owner: String,
    pub max_n: Option<u64>,
    pub verifying_key: Binary,
    pub sp1_vkey_hash: String,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
struct Config {
    owner: Addr,
    max_n: Option<u64>,
    verifying_key: Binary,
    sp1_vkey_hash: String,
}

#[derive(Error, Debug, PartialEq)]
pub enum ContractError {
    #[error("{0}")]
    Std(#[from] StdError),
    #[error("Requested n {n} exceeds contract limit {max}")]
    NTooLarge { n: u64, max: u64 },
    #[error("Fibonacci computation overflowed for n = {n}")]
    Overflow { n: u64 },
    #[error("Failed to deserialize Groth16 verifying key")]
    InvalidVerifyingKey,
    #[error("Invalid SP1 verifying key hash")]
    InvalidSp1VkeyHash,
    #[error("Failed to deserialize Groth16 proof")]
    InvalidProofEncoding,
    #[error("Public values mismatch with provided arguments")]
    PublicValuesMismatch,
    #[error("Groth16 proof verification failed")]
    ProofVerificationFailed,
}

#[entry_point]
pub fn instantiate(
    deps: DepsMut,
    _env: Env,
    info: MessageInfo,
    msg: InstantiateMsg,
) -> Result<Response, ContractError> {
    validate_sp1_vkey_hash(&msg.sp1_vkey_hash)?;
    let config = Config {
        owner: info.sender,
        max_n: msg.max_n,
        verifying_key: msg.verifying_key,
        sp1_vkey_hash: msg.sp1_vkey_hash,
    };
    CONFIG.save(deps.storage, &config)?;
    Ok(Response::new().add_attribute("method", "instantiate"))
}

#[entry_point]
pub fn execute(
    deps: DepsMut,
    _env: Env,
    _info: MessageInfo,
    msg: ExecuteMsg,
) -> Result<Response, ContractError> {
    match msg {
        ExecuteMsg::Verify { n, value, proof } => execute_verify(deps, n, value, proof),
    }
}

fn execute_verify(
    deps: DepsMut,
    n: u64,
    claimed_value: Uint128,
    proof_bytes: Binary,
) -> Result<Response, ContractError> {
    let config = CONFIG.load(deps.storage)?;
    if let Some(max_n) = config.max_n {
        if n > max_n {
            return Err(ContractError::NTooLarge { n, max: max_n });
        }
    }

    if config.verifying_key.is_empty() {
        return Err(ContractError::InvalidVerifyingKey);
    }
    if proof_bytes.is_empty() {
        return Err(ContractError::InvalidProofEncoding);
    }

    let public_values = build_public_values_bytes(n, claimed_value)?;
    Groth16Verifier::verify(
        proof_bytes.as_slice(),
        public_values.as_slice(),
        config.sp1_vkey_hash.as_str(),
        config.verifying_key.as_slice(),
    )
    .map_err(map_groth16_error)?;

    Ok(Response::new()
        .add_attribute("action", "verify")
        .add_attribute("n", n.to_string())
        .add_attribute("valid", "true"))
}

#[entry_point]
pub fn query(deps: Deps, _env: Env, msg: QueryMsg) -> StdResult<Binary> {
    match msg {
        QueryMsg::Expected { n } => {
            let value = fibonacci(n).map(Uint128::from).map_err(|err| match err {
                ContractError::Overflow { n } => {
                    StdError::generic_err(format!("Fibonacci overflow at n={n}"))
                }
                ContractError::NTooLarge { .. } => {
                    StdError::generic_err("unexpected limit error in query")
                }
                ContractError::Std(err) => err,
                _ => StdError::generic_err("unexpected verification error in query"),
            })?;
            to_json_binary(&ExpectedResponse { value })
        }
        QueryMsg::Config {} => {
            let cfg = CONFIG.load(deps.storage)?;
            to_json_binary(&ConfigResponse {
                owner: cfg.owner.into_string(),
                max_n: cfg.max_n,
                verifying_key: cfg.verifying_key,
                sp1_vkey_hash: cfg.sp1_vkey_hash,
            })
        }
    }
}

fn map_groth16_error(err: Groth16Error) -> ContractError {
    match err {
        Groth16Error::ProofVerificationFailed => ContractError::ProofVerificationFailed,
        Groth16Error::ProcessVerifyingKeyFailed => ContractError::InvalidVerifyingKey,
        Groth16Error::PrepareInputsFailed => ContractError::InvalidProofEncoding,
        Groth16Error::GeneralError(_) => ContractError::InvalidProofEncoding,
        Groth16Error::Groth16VkeyHashMismatch => ContractError::ProofVerificationFailed,
    }
}

fn build_public_values_bytes(n: u64, claimed_value: Uint128) -> Result<Vec<u8>, ContractError> {
    if n > u32::MAX as u64 {
        return Err(ContractError::NTooLarge {
            n,
            max: u32::MAX as u64,
        });
    }
    let n_u32 = n as u32;

    let claimed_u32: u32 = claimed_value
        .u128()
        .try_into()
        .map_err(|_| ContractError::PublicValuesMismatch)?;

    let (fib_n, fib_next) = fibonacci_mod_sequence(n_u32);
    if claimed_u32 != fib_n {
        return Err(ContractError::PublicValuesMismatch);
    }

    let mut bytes = Vec::with_capacity(12);
    bytes.extend_from_slice(&n_u32.to_le_bytes());
    bytes.extend_from_slice(&fib_n.to_le_bytes());
    bytes.extend_from_slice(&fib_next.to_le_bytes());
    Ok(bytes)
}

fn fibonacci_mod_sequence(n: u32) -> (u32, u32) {
    let mut a: u32 = 0;
    let mut b: u32 = 1;
    for _ in 0..n {
        let next = ((a as u64 + b as u64) % FIB_MODULUS as u64) as u32;
        a = b;
        b = next;
    }
    (a % FIB_MODULUS, b % FIB_MODULUS)
}

fn validate_sp1_vkey_hash(hash: &str) -> Result<(), ContractError> {
    if !(hash.starts_with("0x")
        && hash.len() == 66
        && hash.chars().skip(2).all(|c| c.is_ascii_hexdigit()))
    {
        return Err(ContractError::InvalidSp1VkeyHash);
    }
    Ok(())
}

fn fibonacci(n: u64) -> Result<u128, ContractError> {
    match n {
        0 => Ok(0),
        1 => Ok(1),
        _ => {
            let mut prev: u128 = 0;
            let mut curr: u128 = 1;
            for i in 2..=n {
                let next = prev
                    .checked_add(curr)
                    .ok_or(ContractError::Overflow { n: i })?;
                prev = curr;
                curr = next;
            }
            Ok(curr)
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use cosmwasm_std::testing::{mock_dependencies, mock_env, mock_info};
    use cosmwasm_std::Binary;

    const DUMMY_HASH: &str = "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";

    #[test]
    fn instantiate_and_config_query() {
        let mut deps = mock_dependencies();
        let info = mock_info("owner", &[]);
        let msg = InstantiateMsg {
            max_n: Some(50),
            verifying_key: Binary::default(),
            sp1_vkey_hash: DUMMY_HASH.to_string(),
        };

        instantiate(deps.as_mut(), mock_env(), info, msg).unwrap();

        let res = query(deps.as_ref(), mock_env(), QueryMsg::Config {}).unwrap();
        let cfg: ConfigResponse = cosmwasm_std::from_binary(&res).unwrap();
        assert_eq!(cfg.owner, "owner");
        assert_eq!(cfg.max_n, Some(50));
        assert_eq!(cfg.verifying_key, Binary::default());
        assert_eq!(cfg.sp1_vkey_hash, DUMMY_HASH);
    }

    #[test]
    fn verify_fails_with_invalid_verifying_key() {
        let mut deps = mock_dependencies();
        let info = mock_info("owner", &[]);
        instantiate(
            deps.as_mut(),
            mock_env(),
            info,
            InstantiateMsg {
                max_n: None,
                verifying_key: Binary::default(),
                sp1_vkey_hash: DUMMY_HASH.to_string(),
            },
        )
        .unwrap();

        let err = execute(
            deps.as_mut(),
            mock_env(),
            mock_info("caller", &[]),
            ExecuteMsg::Verify {
                n: 10,
                value: Uint128::from(55u128),
                proof: Binary::default(),
            },
        )
        .unwrap_err();
        assert_eq!(err, ContractError::InvalidVerifyingKey);
    }

    #[test]
    fn verify_failure() {
        let mut deps = mock_dependencies();
        let info = mock_info("owner", &[]);
        instantiate(
            deps.as_mut(),
            mock_env(),
            info,
            InstantiateMsg {
                max_n: Some(20),
                verifying_key: Binary::default(),
                sp1_vkey_hash: DUMMY_HASH.to_string(),
            },
        )
        .unwrap();

        let err = execute(
            deps.as_mut(),
            mock_env(),
            mock_info("caller", &[]),
            ExecuteMsg::Verify {
                n: 21,
                value: Uint128::from(17711u128),
                proof: Binary::default(),
            },
        )
        .unwrap_err();
        assert_eq!(err, ContractError::NTooLarge { n: 21, max: 20 });

        let err = execute(
            deps.as_mut(),
            mock_env(),
            mock_info("caller", &[]),
            ExecuteMsg::Verify {
                n: 5,
                value: Uint128::from(99u128),
                proof: Binary::default(),
            },
        )
        .unwrap_err();
        assert_eq!(err, ContractError::InvalidVerifyingKey);
    }

    #[test]
    fn fibonacci_overflow_detection() {
        let err = fibonacci(187).unwrap_err();
        assert_eq!(err, ContractError::Overflow { n: 187 });
    }

    #[test]
    fn build_public_values_matches_fixture() {
        let bytes = build_public_values_bytes(20, Uint128::from(6765u128)).unwrap();
        assert_eq!(
            bytes,
            vec![0x14, 0, 0, 0, 0x6d, 0x1a, 0, 0, 0xd3, 0x0b, 0, 0]
        );
    }

    #[test]
    fn fibonacci_mod_sequence_matches_program() {
        let (fib_n, fib_next) = fibonacci_mod_sequence(20);
        assert_eq!(fib_n, 6765);
        assert_eq!(fib_next, 3027);
    }
}
