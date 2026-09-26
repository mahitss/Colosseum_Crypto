// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {ProphetTest} from "../src/ProphetTest.sol";

contract ProphetTestTest {
    function testValueIsOne() public {
        ProphetTest testContract = new ProphetTest();
        require(testContract.value() == 1, "unexpected initial value");
    }
}
