//! Integration tests for genesis JSON parsing with QKC extensions

use reth_optimism_chainspec::OpChainSpec;
use serde_json::json;

#[test]
fn test_genesis_parsing_with_qkc_extension() {
    let genesis_json = json!({
        "config": {
            "chainId": 3335,
            "homesteadBlock": 0,
            "eip150Block": 0,
            "eip155Block": 0,
            "eip158Block": 0,
            "byzantiumBlock": 0,
            "constantinopleBlock": 0,
            "petersburgBlock": 0,
            "istanbulBlock": 0,
            "muirGlacierBlock": 0,
            "berlinBlock": 0,
            "londonBlock": 0,
            "bedrockBlock": 0,
            "regolithTime": 0,
            "canyonTime": 1704992401,
            "optimism": {
                "eip1559Elasticity": 6,
                "eip1559Denominator": 50
            },
            "qkc_extension": {
                "sgtActivationTimestamp": 1735689600,
                "sgtIsNativeBacked": true
            }
        },
        "nonce": "0x0",
        "timestamp": "0x0",
        "extraData": "0x",
        "gasLimit": "0x1c9c380",
        "difficulty": "0x0",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "coinbase": "0x0000000000000000000000000000000000000000",
        "alloc": {},
        "number": "0x0",
        "gasUsed": "0x0",
        "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "baseFeePerGas": "0x3b9aca00"
    });

    let genesis_str = serde_json::to_string(&genesis_json).unwrap();
    let chainspec = OpChainSpec::from_genesis(
        serde_json::from_str(&genesis_str).expect("Failed to parse genesis JSON"),
    );

    // Verify QKC extension was parsed correctly
    assert_eq!(
        chainspec.qkc_extension.sgt_activation_timestamp,
        Some(1735689600),
        "SGT activation timestamp should be parsed"
    );
    assert!(
        chainspec.qkc_extension.sgt_is_native_backed,
        "SGT should be native-backed"
    );
    assert_eq!(
        chainspec.qkc_extension.sgt_contract_address,
        None,
        "SGT contract address should use default"
    );
}

#[test]
fn test_genesis_parsing_with_qkc_extension_camel_case() {
    // Test with camelCase field names (alternative format)
    let genesis_json = json!({
        "config": {
            "chainId": 3335,
            "bedrockBlock": 0,
            "optimism": {
                "eip1559Elasticity": 6,
                "eip1559Denominator": 50
            },
            "qkcExtension": {
                "sgtActivationTimestamp": 1735689600,
                "sgtIsNativeBacked": false,
                "sgtContractAddress": "0x4200000000000000000000000000000000000800"
            }
        },
        "nonce": "0x0",
        "timestamp": "0x0",
        "extraData": "0x",
        "gasLimit": "0x1c9c380",
        "difficulty": "0x0",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "coinbase": "0x0000000000000000000000000000000000000000",
        "alloc": {},
        "number": "0x0",
        "gasUsed": "0x0",
        "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "baseFeePerGas": "0x3b9aca00"
    });

    let genesis_str = serde_json::to_string(&genesis_json).unwrap();
    let chainspec = OpChainSpec::from_genesis(
        serde_json::from_str(&genesis_str).expect("Failed to parse genesis JSON"),
    );

    // Verify QKC extension was parsed correctly with camelCase
    assert_eq!(
        chainspec.qkc_extension.sgt_activation_timestamp,
        Some(1735689600),
        "SGT activation timestamp should be parsed from camelCase"
    );
    assert!(
        !chainspec.qkc_extension.sgt_is_native_backed,
        "SGT should not be native-backed"
    );
    assert_eq!(
        chainspec.qkc_extension.sgt_contract_address,
        Some(alloy_primitives::address!("4200000000000000000000000000000000000800")),
        "SGT contract address should be parsed"
    );
}

#[test]
fn test_genesis_parsing_without_qkc_extension() {
    // Test that genesis without QKC extension uses defaults
    let genesis_json = json!({
        "config": {
            "chainId": 10,
            "bedrockBlock": 0,
            "optimism": {
                "eip1559Elasticity": 6,
                "eip1559Denominator": 50
            }
        },
        "nonce": "0x0",
        "timestamp": "0x0",
        "extraData": "0x",
        "gasLimit": "0x1c9c380",
        "difficulty": "0x0",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "coinbase": "0x0000000000000000000000000000000000000000",
        "alloc": {},
        "number": "0x0",
        "gasUsed": "0x0",
        "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "baseFeePerGas": "0x3b9aca00"
    });

    let genesis_str = serde_json::to_string(&genesis_json).unwrap();
    let chainspec = OpChainSpec::from_genesis(
        serde_json::from_str(&genesis_str).expect("Failed to parse genesis JSON"),
    );

    // Verify QKC extension defaults
    assert_eq!(
        chainspec.qkc_extension.sgt_activation_timestamp,
        None,
        "SGT should be disabled by default"
    );
    assert!(
        chainspec.qkc_extension.sgt_is_native_backed,
        "SGT should default to native-backed mode"
    );
    assert_eq!(
        chainspec.qkc_extension.sgt_contract_address,
        None,
        "SGT contract address should be None"
    );
}

#[test]
fn test_is_sgt_active_at_timestamp() {
    let genesis_json = json!({
        "config": {
            "chainId": 3335,
            "bedrockBlock": 0,
            "optimism": {
                "eip1559Elasticity": 6,
                "eip1559Denominator": 50
            },
            "qkc_extension": {
                "sgtActivationTimestamp": 1000000
            }
        },
        "nonce": "0x0",
        "timestamp": "0x0",
        "extraData": "0x",
        "gasLimit": "0x1c9c380",
        "difficulty": "0x0",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "coinbase": "0x0000000000000000000000000000000000000000",
        "alloc": {},
        "number": "0x0",
        "gasUsed": "0x0",
        "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "baseFeePerGas": "0x3b9aca00"
    });

    let genesis_str = serde_json::to_string(&genesis_json).unwrap();
    let chainspec = OpChainSpec::from_genesis(
        serde_json::from_str(&genesis_str).expect("Failed to parse genesis JSON"),
    );

    // Test activation timestamp logic
    assert!(
        !chainspec.is_sgt_active_at_timestamp(999999),
        "SGT should not be active before activation timestamp"
    );
    assert!(
        chainspec.is_sgt_active_at_timestamp(1000000),
        "SGT should be active at activation timestamp"
    );
    assert!(
        chainspec.is_sgt_active_at_timestamp(1000001),
        "SGT should be active after activation timestamp"
    );
}

#[test]
fn test_sgt_contract_address_default() {
    let genesis_json = json!({
        "config": {
            "chainId": 3335,
            "bedrockBlock": 0,
            "optimism": {
                "eip1559Elasticity": 6,
                "eip1559Denominator": 50
            },
            "qkc_extension": {
                "sgtActivationTimestamp": 1000000
            }
        },
        "nonce": "0x0",
        "timestamp": "0x0",
        "extraData": "0x",
        "gasLimit": "0x1c9c380",
        "difficulty": "0x0",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "coinbase": "0x0000000000000000000000000000000000000000",
        "alloc": {},
        "number": "0x0",
        "gasUsed": "0x0",
        "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "baseFeePerGas": "0x3b9aca00"
    });

    let genesis_str = serde_json::to_string(&genesis_json).unwrap();
    let chainspec = OpChainSpec::from_genesis(
        serde_json::from_str(&genesis_str).expect("Failed to parse genesis JSON"),
    );

    // Verify default contract address
    assert_eq!(
        chainspec.sgt_contract_address(),
        alloy_primitives::address!("4200000000000000000000000000000000000800"),
        "Should return default SGT contract address"
    );
}

#[test]
fn test_sgt_contract_address_override() {
    let custom_address = "0x1234567890123456789012345678901234567890";
    let genesis_json = json!({
        "config": {
            "chainId": 3335,
            "bedrockBlock": 0,
            "optimism": {
                "eip1559Elasticity": 6,
                "eip1559Denominator": 50
            },
            "qkc_extension": {
                "sgtActivationTimestamp": 1000000,
                "sgtContractAddress": custom_address
            }
        },
        "nonce": "0x0",
        "timestamp": "0x0",
        "extraData": "0x",
        "gasLimit": "0x1c9c380",
        "difficulty": "0x0",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "coinbase": "0x0000000000000000000000000000000000000000",
        "alloc": {},
        "number": "0x0",
        "gasUsed": "0x0",
        "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "baseFeePerGas": "0x3b9aca00"
    });

    let genesis_str = serde_json::to_string(&genesis_json).unwrap();
    let chainspec = OpChainSpec::from_genesis(
        serde_json::from_str(&genesis_str).expect("Failed to parse genesis JSON"),
    );

    // Verify custom contract address
    assert_eq!(
        chainspec.sgt_contract_address(),
        alloy_primitives::address!("1234567890123456789012345678901234567890"),
        "Should return custom SGT contract address"
    );
}
