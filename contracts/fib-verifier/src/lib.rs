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

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct InstantiateMsg {
    /// Optional upper bound on the Fibonacci index that can be verified.
    pub max_n: Option<u64>,
    /// SP1 Groth16 verifying key bytes (compressed, BN254 curve).
    pub verifying_key: Binary,
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
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
struct Config {
    owner: Addr,
    max_n: Option<u64>,
    verifying_key: Binary,
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
    #[error("Failed to deserialize Groth16 proof")]
    InvalidProofEncoding,
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
    let config = Config {
        owner: info.sender,
        max_n: msg.max_n,
        verifying_key: msg.verifying_key,
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

    let public_inputs = build_public_inputs(n, claimed_value);
    Groth16Verifier::verify_gnark_proof(
        proof_bytes.as_slice(),
        &public_inputs,
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
                ContractError::Overflow { n } =>
                    StdError::generic_err(format!("Fibonacci overflow at n={n}")),
                ContractError::NTooLarge { .. } => StdError::generic_err("unexpected limit error in query"),
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

fn build_public_inputs(n: u64, value: Uint128) -> [[u8; 32]; 2] {
    [u64_to_field_bytes(n), u128_to_field_bytes(value.u128())]
}

fn u64_to_field_bytes(value: u64) -> [u8; 32] {
    let mut bytes = [0u8; 32];
    bytes[24..].copy_from_slice(&value.to_be_bytes());
    bytes
}

fn u128_to_field_bytes(value: u128) -> [u8; 32] {
    let mut bytes = [0u8; 32];
    bytes[16..].copy_from_slice(&value.to_be_bytes());
    bytes
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

    #[test]
    fn instantiate_and_config_query() {
        let mut deps = mock_dependencies();
        let info = mock_info("owner", &[]);
        let msg = InstantiateMsg {
            max_n: Some(50),
            verifying_key: Binary::default(),
        };

        instantiate(deps.as_mut(), mock_env(), info, msg).unwrap();

        let res = query(deps.as_ref(), mock_env(), QueryMsg::Config {}).unwrap();
        let cfg: ConfigResponse = cosmwasm_std::from_binary(&res).unwrap();
        assert_eq!(cfg.owner, "owner");
        assert_eq!(cfg.max_n, Some(50));
        assert_eq!(cfg.verifying_key, Binary::default());
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
}
