// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Test} from "forge-std/Test.sol";
import {QDayAnchorRegistry} from "../src/QDayAnchorRegistry.sol";

contract QDayAnchorRegistryTest is Test {
    QDayAnchorRegistry internal registry;

    address internal owner = address(this);
    address internal relayer = makeAddr("relayer");
    address internal stranger = makeAddr("stranger");

    uint64 internal constant BATCH = 1024;
    bytes32 internal constant ROOT =
        0x3fa8000000000000000000000000000000000000000000000000000000000c21;
    bytes32 internal constant ABEL_TX =
        0xabcdef0000000000000000000000000000000000000000000000000000000001;
    uint64 internal constant ABEL_HEIGHT = 987654;

    event Anchored(uint64 indexed batchNumber, bytes32 indexed stateRoot,
                   bytes32 abelTxHash, uint64 abelHeight);
    event RelayerSet(address relayer, bool allowed);

    function setUp() public {
        registry = new QDayAnchorRegistry();
        registry.setRelayer(relayer, true);
    }

    // ── 權限 ──────────────────────────────────────────────

    function test_SetRelayer_OnlyOwner() public {
        vm.prank(stranger);
        vm.expectRevert(QDayAnchorRegistry.NotOwner.selector);
        registry.setRelayer(stranger, true);
    }

    function test_SetRelayer_EmitsEvent() public {
        vm.expectEmit(false, false, false, true);
        emit RelayerSet(stranger, true);
        registry.setRelayer(stranger, true);
        assertTrue(registry.relayers(stranger));

        registry.setRelayer(stranger, false);
        assertFalse(registry.relayers(stranger));
    }

    function test_RecordAnchor_RevertNotRelayer() public {
        vm.prank(stranger);
        vm.expectRevert(QDayAnchorRegistry.NotRelayer.selector);
        registry.recordAnchor(BATCH, ROOT, ABEL_TX, ABEL_HEIGHT);
    }

    // ── event 內容 ────────────────────────────────────────

    function test_RecordAnchor_EmitsAnchored() public {
        vm.expectEmit(true, true, false, true);
        emit Anchored(BATCH, ROOT, ABEL_TX, ABEL_HEIGHT);
        vm.prank(relayer);
        registry.recordAnchor(BATCH, ROOT, ABEL_TX, ABEL_HEIGHT);
    }

    // ── anchorOf 回讀 ─────────────────────────────────────

    function test_AnchorOf_ReadsBack() public {
        vm.warp(1_751_800_000);
        vm.prank(relayer);
        registry.recordAnchor(BATCH, ROOT, ABEL_TX, ABEL_HEIGHT);

        (uint64 batchNumber, bytes32 abelTxHash, uint64 abelHeight, uint64 recordedAt) =
            registry.anchorOf(ROOT);
        assertEq(batchNumber, BATCH);
        assertEq(abelTxHash, ABEL_TX);
        assertEq(abelHeight, ABEL_HEIGHT);
        assertEq(recordedAt, uint64(1_751_800_000));

        assertTrue(registry.isAnchored(ROOT));
        assertEq(registry.rootOfBatch(BATCH), ROOT);
    }

    function test_AnchorOf_EmptyForUnknownRoot() public view {
        (uint64 batchNumber,, uint64 abelHeight, uint64 recordedAt) =
            registry.anchorOf(bytes32(uint256(1)));
        assertEq(batchNumber, 0);
        assertEq(abelHeight, 0);
        assertEq(recordedAt, 0);
        assertFalse(registry.isAnchored(bytes32(uint256(1))));
    }

    // ── 重複 revert ───────────────────────────────────────

    function test_RecordAnchor_RevertDuplicateRoot() public {
        vm.startPrank(relayer);
        registry.recordAnchor(BATCH, ROOT, ABEL_TX, ABEL_HEIGHT);
        vm.expectRevert(abi.encodeWithSelector(QDayAnchorRegistry.AlreadyAnchored.selector, ROOT));
        registry.recordAnchor(BATCH + 1, ROOT, ABEL_TX, ABEL_HEIGHT);
        vm.stopPrank();
    }

    function test_RecordAnchor_RevertDuplicateBatch() public {
        vm.startPrank(relayer);
        registry.recordAnchor(BATCH, ROOT, ABEL_TX, ABEL_HEIGHT);
        vm.expectRevert(abi.encodeWithSelector(QDayAnchorRegistry.BatchAlreadyAnchored.selector, BATCH));
        registry.recordAnchor(BATCH, bytes32(uint256(2)), ABEL_TX, ABEL_HEIGHT);
        vm.stopPrank();
    }
}
