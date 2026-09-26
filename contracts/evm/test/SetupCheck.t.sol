// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {SetupCheck} from "../src/SetupCheck.sol";

contract SetupCheckTest {
    function testFoundrySetup() public {
        SetupCheck instance = new SetupCheck();
        require(address(instance) != address(0), "deployment failed");
    }
}