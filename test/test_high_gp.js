// 简化版高Gas Price交易测试
const { ethers } = require("ethers");
const fs = require("fs");

async function runTest() {
  try {
    console.log("开始高Gas Price交易测试...");
    
    // 连接到RPC节点
    const provider = new ethers.providers.JsonRpcProvider("http://localhost:8123");

