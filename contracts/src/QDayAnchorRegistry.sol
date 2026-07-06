// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

contract QDayAnchorRegistry {
    struct Anchor {
        uint64  batchNumber;
        bytes32 abelTxHash;
        uint64  abelHeight;
        uint64  recordedAt;   // block.timestamp
    }

    address public owner;
    mapping(address => bool) public relayers;
    mapping(bytes32 => Anchor) private anchors;       // stateRoot => Anchor
    mapping(uint64  => bytes32) public rootOfBatch;   // batchNumber => stateRoot

    event Anchored(uint64 indexed batchNumber, bytes32 indexed stateRoot,
                   bytes32 abelTxHash, uint64 abelHeight);
    event RelayerSet(address relayer, bool allowed);

    error NotOwner();
    error NotRelayer();
    error AlreadyAnchored(bytes32 stateRoot);
    error BatchAlreadyAnchored(uint64 batchNumber);

    modifier onlyOwner()   { if (msg.sender != owner) revert NotOwner(); _; }
    modifier onlyRelayer() { if (!relayers[msg.sender]) revert NotRelayer(); _; }

    constructor() { owner = msg.sender; }

    function setRelayer(address r, bool allowed) external onlyOwner {
        relayers[r] = allowed;
        emit RelayerSet(r, allowed);
    }

    function recordAnchor(uint64 batchNumber, bytes32 stateRoot,
                          bytes32 abelTxHash, uint64 abelHeight)
        external onlyRelayer
    {
        if (anchors[stateRoot].recordedAt != 0) revert AlreadyAnchored(stateRoot);
        if (rootOfBatch[batchNumber] != bytes32(0)) revert BatchAlreadyAnchored(batchNumber);
        anchors[stateRoot] = Anchor(batchNumber, abelTxHash, abelHeight, uint64(block.timestamp));
        rootOfBatch[batchNumber] = stateRoot;
        emit Anchored(batchNumber, stateRoot, abelTxHash, abelHeight);
    }

    function anchorOf(bytes32 stateRoot) external view
        returns (uint64 batchNumber, bytes32 abelTxHash, uint64 abelHeight, uint64 recordedAt)
    {
        Anchor memory a = anchors[stateRoot];
        return (a.batchNumber, a.abelTxHash, a.abelHeight, a.recordedAt);
    }

    function isAnchored(bytes32 stateRoot) external view returns (bool) {
        return anchors[stateRoot].recordedAt != 0;
    }
}
