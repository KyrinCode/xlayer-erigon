// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract BlockhashDemo {
    // Store the block hashes
    bytes32[] public blockHashes;

    // Test function to store the current block hash
    function test() public {
        // Fetch the hash of the latest block
        bytes32 currentBlockHash = blockhash(block.number - 1);

        // Store the hash on-chain
        blockHashes.push(currentBlockHash);
    }

    // Retrieve a stored block hash by index
    function getBlockHash(uint256 index) public view returns (bytes32) {
        require(index < blockHashes.length, "Index out of bounds");
        return blockHashes[index];
    }

    // Retrieve the total count of stored block hashes
    function getBlockHashCount() public view returns (uint256) {
        return blockHashes.length;
    }
} 