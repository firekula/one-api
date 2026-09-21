export function getLastSevenDays() {
  const dates = [];
  for (let i = 6; i >= 0; i--) {
    const d = new Date();
    d.setDate(d.getDate() - i);
    const month = '' + (d.getMonth() + 1);
    const day = '' + d.getDate();
    const year = d.getFullYear();

    const formattedDate = [year, month.padStart(2, '0'), day.padStart(2, '0')].join('-');
    dates.push(formattedDate);
  }
  return dates;
}

// 统一把 Date / 字符串转成本地时区的 Date。
// 「YYYY-MM-DD HH:mm:ss」这种带空格的写法在部分浏览器（如 Safari）里 Date 无法解析，
// 换成 ISO 的「YYYY-MM-DDTHH:mm:ss」；裸日期串（YYYY-MM-DD）会被按 UTC 午夜解析，
// 补上「T00:00:00」后才是本地自然日。解析失败返回 Invalid Date，调用方得到的桶列表为空。
function toLocalDate(value) {
  if (value instanceof Date) {
    return new Date(value);
  }
  const text = String(value === null || value === undefined ? '' : value).trim();
  if (text === '') {
    return new Date(NaN);
  }
  const iso = text.replace(' ', 'T');
  return new Date(iso.includes('T') ? iso : `${iso}T00:00:00`);
}

// 按请求区间与粒度生成桶标签，本地时区，与后端分桶口径一致。
// 用 Date 的日历加法（setDate / setHours）推进而非固定毫秒步长，夏令时切换时标签不会偏移。
export function getDateRange(startDate, endDate, granularity = 'day') {
  const pad = (n) => String(n).padStart(2, '0');
  const format = (d) =>
    granularity === 'hour'
      ? `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:00`
      : `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
  const dates = [];
  const cursor = toLocalDate(startDate);
  if (granularity === 'hour') {
    cursor.setMinutes(0, 0, 0);
  } else {
    cursor.setHours(0, 0, 0, 0);
  }
  const end = toLocalDate(endDate);
  let guard = 0;
  while (cursor <= end && guard < 4000) {
    dates.push(format(cursor));
    if (granularity === 'hour') {
      cursor.setHours(cursor.getHours() + 1);
    } else {
      cursor.setDate(cursor.getDate() + 1);
    }
    guard += 1;
  }
  return dates;
}

export function getTodayDay() {
  let today = new Date();
  return today.toISOString().slice(0, 10);
}

export function generateChartOptions(data, unit) {
  const dates = data.map((item) => item.date);
  const values = data.map((item) => item.value);

  const minDate = dates[0];
  const maxDate = dates[dates.length - 1];

  const minValue = Math.min(...values);
  const maxValue = Math.max(...values);

  return {
    series: [
      {
        data: values
      }
    ],
    type: 'line',
    height: 90,
    options: {
      chart: {
        sparkline: {
          enabled: true
        },
        background: 'transparent'
      },
      dataLabels: {
        enabled: false
      },
      colors: ['#fff'],
      fill: {
        type: 'solid',
        opacity: 1
      },
      stroke: {
        curve: 'smooth',
        width: 3
      },
      xaxis: {
        categories: dates,
        labels: {
          show: false
        },
        min: minDate,
        max: maxDate
      },
      yaxis: {
        min: minValue,
        max: maxValue,
        labels: {
          show: false
        }
      },
      tooltip: {
        theme: 'dark',
        fixed: {
          enabled: false
        },
        x: {
          format: 'yyyy-MM-dd'
        },
        y: {
          formatter: function (val) {
            return val + ` ${unit}`;
          },
          title: {
            formatter: function () {
              return '';
            }
          }
        },
        marker: {
          show: false
        }
      }
    }
  };
}
