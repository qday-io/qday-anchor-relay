// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// Dev-only stand-in for the QDAY2 commit contract (real ABI: TO-CONFIRM C1).
/// Emits the event shape the relay's EventLogCommitSource assumes, so the
/// poller can be exercised end-to-end before the real contract is known.
contract MockBatchCommitter {
    event BatchCommitted(uint64 indexed batchNumber, bytes32 stateRoot);

    function commit(uint64 batchNumber, bytes32 stateRoot) external {
        emit BatchCommitted(batchNumber, stateRoot);
    }
}
