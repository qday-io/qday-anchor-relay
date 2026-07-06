// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Script, console} from "forge-std/Script.sol";
import {QDayAnchorRegistry} from "../src/QDayAnchorRegistry.sol";

/// Deploys QDayAnchorRegistry and authorizes the relayer EOA.
///
/// Usage:
///   RELAYER_ADDRESS=0x... forge script script/Deploy.s.sol \
///     --rpc-url $DA_RPC_URL --private-key $DEPLOYER_PRIVATE_KEY --broadcast
///
/// If RELAYER_ADDRESS is unset, the broadcasting sender is authorized as relayer.
contract Deploy is Script {
    function run() external {
        vm.startBroadcast();
        QDayAnchorRegistry registry = new QDayAnchorRegistry();

        address relayer = vm.envOr("RELAYER_ADDRESS", msg.sender);
        registry.setRelayer(relayer, true);
        vm.stopBroadcast();

        console.log("QDayAnchorRegistry deployed at:", address(registry));
        console.log("Relayer authorized:", relayer);
    }
}
