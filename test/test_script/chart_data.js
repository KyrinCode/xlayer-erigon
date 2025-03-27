// XLayer Gas价格分析图表数据
// 生成时间: 2025/3/26 10:17:37

const chartData = {
  "confirmationTime": {
    "labels": [
      "0.5x",
      "1x",
      "2x",
      "5x",
      "10x"
    ],
    "datasets": [
      {
        "label": "平均确认时间(秒)",
        "data": [
          "12.03",
          "13.09",
          "4.06",
          "12.04",
          "4.01"
        ]
      },
      {
        "label": "最短确认时间(秒)",
        "data": [
          "12.03",
          "13.09",
          "4.06",
          "12.04",
          "4.01"
        ]
      },
      {
        "label": "最长确认时间(秒)",
        "data": [
          "12.03",
          "13.09",
          "4.06",
          "12.04",
          "4.01"
        ]
      }
    ]
  },
  "gasPrice": {
    "labels": [
      "0.5x",
      "1x",
      "2x",
      "5x",
      "10x"
    ],
    "expected": [
      250,
      500,
      1000,
      2500,
      5000
    ],
    "actual": [
      250,
      500,
      1000,
      2500,
      5000
    ]
  },
  "scatterData": [
    {
      "x": 0.5,
      "y": "12.03",
      "r": 7,
      "label": "0.5x (250 Gwei)"
    },
    {
      "x": 1,
      "y": "13.09",
      "r": 7,
      "label": "1x (500 Gwei)"
    },
    {
      "x": 2,
      "y": "4.06",
      "r": 7,
      "label": "2x (1000 Gwei)"
    },
    {
      "x": 5,
      "y": "12.04",
      "r": 7,
      "label": "5x (2500 Gwei)"
    },
    {
      "x": 10,
      "y": "4.01",
      "r": 7,
      "label": "10x (5000 Gwei)"
    }
  ]
};

// 使用示例:
/*
// 使用Chart.js创建图表
const ctx = document.getElementById('confirmationTimeChart').getContext('2d');
new Chart(ctx, {
  type: 'bar',
  data: {
    labels: chartData.confirmationTime.labels,
    datasets: chartData.confirmationTime.datasets
  },
  options: {
    responsive: true,
    title: {
      display: true,
      text: 'Gas价格倍数与确认时间关系'
    }
  }
});

// 创建散点图
const scatterCtx = document.getElementById('scatterChart').getContext('2d');
new Chart(scatterCtx, {
  type: 'bubble',
  data: {
    datasets: [{
      label: 'Gas价格vs确认时间',
      data: chartData.scatterData
    }]
  },
  options: {
    scales: {
      x: {
        title: {
          display: true,
          text: 'Gas价格倍数'
        }
      },
      y: {
        title: {
          display: true,
          text: '确认时间(秒)'
        }
      }
    }
  }
});
*/
