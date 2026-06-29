const sampleIntent =
  "upi://pay?am=1258.95&cu=INR&mc=4131&mode=04&pa=redbus608079.rzp@rxairtel&pn=MakemytripIndiaPrivateLimited&tn=MakemytripIndiaPrivateLimitedredBus&tr=T79aKQjNkknOtI";

const intentInput = document.querySelector("#intent");
const qrTarget = document.querySelector("#qr");
const statusTarget = document.querySelector("#status");
const versionTarget = document.querySelector("#version");
const payeeTarget = document.querySelector("#payee");
const amountTarget = document.querySelector("#amount");
const referenceTarget = document.querySelector("#reference");
const downloadLink = document.querySelector("#download");
const openLink = document.querySelector("#open-upi");

const params = new URLSearchParams(window.location.search);
intentInput.value = normalizeIntent(params.get("upi") || params.get("url") || sampleIntent);

document.querySelector("#render").addEventListener("click", render);
document.querySelector("#copy-upi").addEventListener("click", () => copyText(normalizeIntent(intentInput.value), "UPI URL copied"));
document.querySelector("#copy-page").addEventListener("click", () => copyText(makePageLink(), "Page link copied"));
intentInput.addEventListener("input", render);

function render() {
  const value = normalizeIntent(intentInput.value);
  intentInput.value = value;

  try {
    validateIntent(value);
    const qr = QrCode.encode(value);
    const svg = qrToSvg(qr.modules);
    const dataUrl = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;

    qrTarget.innerHTML = svg;
    downloadLink.href = dataUrl;
    openLink.href = value;
    versionTarget.textContent = `QR v${qr.version}, ${qr.modules.length}x${qr.modules.length}`;
    statusTarget.className = "";
    statusTarget.textContent = "Ready";
    updateDetails(value);
    updateAddressBar(value);
  } catch (error) {
    statusTarget.className = "error";
    statusTarget.textContent = error.message;
    versionTarget.textContent = "";
  }
}

function normalizeIntent(value) {
  return String(value || "")
    .trim()
    .replaceAll("\\u0026", "&")
    .replaceAll("&amp;", "&");
}

function validateIntent(value) {
  if (!value) {
    throw new Error("Add a UPI intent URL");
  }
  if (!/^upi:\/\/pay\?/i.test(value)) {
    throw new Error("Expected upi://pay?... URL");
  }
}

function updateDetails(value) {
  const query = value.slice(value.indexOf("?") + 1);
  const intentParams = new URLSearchParams(query);
  const amount = intentParams.get("am");
  const currency = intentParams.get("cu") || "INR";

  payeeTarget.textContent = intentParams.get("pn") || intentParams.get("pa") || "-";
  amountTarget.textContent = amount ? `${currency} ${amount}` : "-";
  referenceTarget.textContent = intentParams.get("tr") || intentParams.get("tn") || "-";
}

function makePageLink() {
  const url = new URL(window.location.href);
  url.search = "";
  url.searchParams.set("upi", normalizeIntent(intentInput.value));
  return url.toString();
}

function updateAddressBar(value) {
  const url = new URL(window.location.href);
  url.search = "";
  url.searchParams.set("upi", value);
  history.replaceState(null, "", url);
}

async function copyText(text, success) {
  try {
    await navigator.clipboard.writeText(text);
  } catch (_) {
    const textArea = document.createElement("textarea");
    textArea.value = text;
    textArea.setAttribute("readonly", "");
    textArea.style.position = "fixed";
    textArea.style.left = "-999px";
    document.body.appendChild(textArea);
    textArea.select();
    document.execCommand("copy");
    textArea.remove();
  }

  statusTarget.className = "";
  statusTarget.textContent = success;
}

function qrToSvg(modules) {
  const quiet = 4;
  const size = modules.length + quiet * 2;
  let path = "";

  for (let y = 0; y < modules.length; y += 1) {
    for (let x = 0; x < modules.length; x += 1) {
      if (modules[y][x]) {
        path += `M${x + quiet},${y + quiet}h1v1h-1z`;
      }
    }
  }

  return [
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${size} ${size}" shape-rendering="crispEdges" aria-hidden="true">`,
    `<rect width="${size}" height="${size}" fill="#fff"/>`,
    `<path fill="#111" d="${path}"/>`,
    "</svg>",
  ].join("");
}

class QrCode {
  static encode(text) {
    const bytes = Array.from(new TextEncoder().encode(text));
    const version = chooseVersion(bytes);
    const data = makeDataCodewords(bytes, version);
    const codewords = addErrorCorrection(data, version);
    const qr = new QrCode(version);

    qr.drawCodewords(codewords);
    const baseModules = cloneMatrix(qr.modules);
    let bestMask = 0;
    let bestPenalty = Infinity;
    let bestModules = null;

    for (let mask = 0; mask < 8; mask += 1) {
      qr.modules = cloneMatrix(baseModules);
      qr.applyMask(mask);
      qr.drawFormatBits(mask);
      const penalty = getPenaltyScore(qr.modules);

      if (penalty < bestPenalty) {
        bestMask = mask;
        bestPenalty = penalty;
        bestModules = cloneMatrix(qr.modules);
      }
    }

    qr.modules = bestModules;
    qr.mask = bestMask;
    return qr;
  }

  constructor(version) {
    this.version = version;
    this.size = version * 4 + 17;
    this.modules = makeMatrix(this.size, false);
    this.isFunction = makeMatrix(this.size, false);
    this.drawFunctionPatterns();
  }

  setFunctionModule(x, y, dark) {
    this.modules[y][x] = dark;
    this.isFunction[y][x] = true;
  }

  drawFunctionPatterns() {
    for (let i = 0; i < this.size; i += 1) {
      this.setFunctionModule(6, i, i % 2 === 0);
      this.setFunctionModule(i, 6, i % 2 === 0);
    }

    this.drawFinderPattern(3, 3);
    this.drawFinderPattern(this.size - 4, 3);
    this.drawFinderPattern(3, this.size - 4);

    const positions = getAlignmentPatternPositions(this.version);
    for (const x of positions) {
      for (const y of positions) {
        if (
          (x === 6 && y === 6) ||
          (x === 6 && y === this.size - 7) ||
          (x === this.size - 7 && y === 6)
        ) {
          continue;
        }
        this.drawAlignmentPattern(x, y);
      }
    }

    this.drawFormatBits(0);
    this.drawVersionBits();
  }

  drawFinderPattern(cx, cy) {
    for (let dy = -4; dy <= 4; dy += 1) {
      for (let dx = -4; dx <= 4; dx += 1) {
        const x = cx + dx;
        const y = cy + dy;
        if (x < 0 || y < 0 || x >= this.size || y >= this.size) {
          continue;
        }

        const dist = Math.max(Math.abs(dx), Math.abs(dy));
        this.setFunctionModule(x, y, dist !== 2 && dist !== 4);
      }
    }
  }

  drawAlignmentPattern(cx, cy) {
    for (let dy = -2; dy <= 2; dy += 1) {
      for (let dx = -2; dx <= 2; dx += 1) {
        this.setFunctionModule(cx + dx, cy + dy, Math.max(Math.abs(dx), Math.abs(dy)) !== 1);
      }
    }
  }

  drawFormatBits(mask) {
    const data = mask;
    let rem = data;

    for (let i = 0; i < 10; i += 1) {
      rem = (rem << 1) ^ ((rem >>> 9) * 0x537);
    }

    const bits = ((data << 10) | rem) ^ 0x5412;

    for (let i = 0; i <= 5; i += 1) {
      this.setFunctionModule(8, i, getBit(bits, i));
    }
    this.setFunctionModule(8, 7, getBit(bits, 6));
    this.setFunctionModule(8, 8, getBit(bits, 7));
    this.setFunctionModule(7, 8, getBit(bits, 8));
    for (let i = 9; i < 15; i += 1) {
      this.setFunctionModule(14 - i, 8, getBit(bits, i));
    }

    for (let i = 0; i < 8; i += 1) {
      this.setFunctionModule(this.size - 1 - i, 8, getBit(bits, i));
    }
    for (let i = 8; i < 15; i += 1) {
      this.setFunctionModule(8, this.size - 15 + i, getBit(bits, i));
    }
    this.setFunctionModule(8, this.size - 8, true);
  }

  drawVersionBits() {
    if (this.version < 7) {
      return;
    }

    let rem = this.version;
    for (let i = 0; i < 12; i += 1) {
      rem = (rem << 1) ^ ((rem >>> 11) * 0x1f25);
    }
    const bits = (this.version << 12) | rem;

    for (let i = 0; i < 18; i += 1) {
      const color = getBit(bits, i);
      const a = this.size - 11 + (i % 3);
      const b = Math.floor(i / 3);
      this.setFunctionModule(a, b, color);
      this.setFunctionModule(b, a, color);
    }
  }

  drawCodewords(data) {
    let i = 0;

    for (let right = this.size - 1; right >= 1; right -= 2) {
      if (right === 6) {
        right = 5;
      }

      for (let vert = 0; vert < this.size; vert += 1) {
        for (let j = 0; j < 2; j += 1) {
          const x = right - j;
          const upward = ((right + 1) & 2) === 0;
          const y = upward ? this.size - 1 - vert : vert;

          if (!this.isFunction[y][x]) {
            this.modules[y][x] = i < data.length * 8 && getBit(data[Math.floor(i / 8)], 7 - (i & 7));
            i += 1;
          }
        }
      }
    }
  }

  applyMask(mask) {
    for (let y = 0; y < this.size; y += 1) {
      for (let x = 0; x < this.size; x += 1) {
        if (!this.isFunction[y][x] && getMaskBit(mask, x, y)) {
          this.modules[y][x] = !this.modules[y][x];
        }
      }
    }
  }
}

function chooseVersion(bytes) {
  for (let version = 1; version <= 40; version += 1) {
    const dataCodewords = getNumDataCodewords(version);
    const countBits = version <= 9 ? 8 : 16;
    const neededBits = 4 + countBits + bytes.length * 8;

    if (neededBits <= dataCodewords * 8) {
      return version;
    }
  }

  throw new Error("UPI URL is too long for a QR code");
}

function makeDataCodewords(bytes, version) {
  const dataCodewords = getNumDataCodewords(version);
  const capacityBits = dataCodewords * 8;
  const bits = [];

  appendBits(bits, 0x4, 4);
  appendBits(bits, bytes.length, version <= 9 ? 8 : 16);
  for (const byte of bytes) {
    appendBits(bits, byte, 8);
  }

  appendBits(bits, 0, Math.min(4, capacityBits - bits.length));
  while (bits.length % 8 !== 0) {
    bits.push(0);
  }

  const result = [];
  for (let i = 0; i < bits.length; i += 8) {
    result.push(parseInt(bits.slice(i, i + 8).join(""), 2));
  }

  for (let pad = 0xec; result.length < dataCodewords; pad ^= 0xec ^ 0x11) {
    result.push(pad);
  }

  return result;
}

function appendBits(bits, value, length) {
  if (length < 0 || value >>> length !== 0) {
    throw new Error("QR encoder bit overflow");
  }
  for (let i = length - 1; i >= 0; i -= 1) {
    bits.push((value >>> i) & 1);
  }
}

function addErrorCorrection(data, version) {
  const numBlocks = NUM_ERROR_CORRECTION_BLOCKS[version];
  const blockEccLen = ECC_CODEWORDS_PER_BLOCK[version];
  const rawCodewords = Math.floor(getNumRawDataModules(version) / 8);
  const numShortBlocks = numBlocks - (rawCodewords % numBlocks);
  const shortBlockLen = Math.floor(rawCodewords / numBlocks);
  const rsDivisor = reedSolomonDivisor(blockEccLen);
  const blocks = [];
  let dataIndex = 0;

  for (let i = 0; i < numBlocks; i += 1) {
    const dataLen = shortBlockLen - blockEccLen + (i < numShortBlocks ? 0 : 1);
    const block = data.slice(dataIndex, dataIndex + dataLen);
    dataIndex += dataLen;
    const ecc = reedSolomonRemainder(block, rsDivisor);

    if (i < numShortBlocks) {
      block.push(0);
    }

    blocks.push(block.concat(ecc));
  }

  const result = [];
  for (let i = 0; i < blocks[0].length; i += 1) {
    for (let j = 0; j < blocks.length; j += 1) {
      if (i !== shortBlockLen - blockEccLen || j >= numShortBlocks) {
        result.push(blocks[j][i]);
      }
    }
  }

  return result;
}

function reedSolomonDivisor(degree) {
  const result = Array(degree).fill(0);
  result[degree - 1] = 1;
  let root = 1;

  for (let i = 0; i < degree; i += 1) {
    for (let j = 0; j < result.length; j += 1) {
      result[j] = reedSolomonMultiply(result[j], root);
      if (j + 1 < result.length) {
        result[j] ^= result[j + 1];
      }
    }
    root = reedSolomonMultiply(root, 0x02);
  }

  return result;
}

function reedSolomonRemainder(data, divisor) {
  const result = Array(divisor.length).fill(0);

  for (const byte of data) {
    const factor = byte ^ result.shift();
    result.push(0);
    for (let i = 0; i < result.length; i += 1) {
      result[i] ^= reedSolomonMultiply(divisor[i], factor);
    }
  }

  return result;
}

function reedSolomonMultiply(x, y) {
  let z = 0;

  for (let i = 7; i >= 0; i -= 1) {
    z = ((z << 1) ^ ((z >>> 7) * 0x11d)) & 0xff;
    z ^= ((y >>> i) & 1) * x;
  }

  return z;
}

function getNumDataCodewords(version) {
  return (
    Math.floor(getNumRawDataModules(version) / 8) -
    ECC_CODEWORDS_PER_BLOCK[version] * NUM_ERROR_CORRECTION_BLOCKS[version]
  );
}

function getNumRawDataModules(version) {
  let result = (16 * version + 128) * version + 64;

  if (version >= 2) {
    const numAlign = Math.floor(version / 7) + 2;
    result -= (25 * numAlign - 10) * numAlign - 55;
    if (version >= 7) {
      result -= 36;
    }
  }

  return result;
}

function getAlignmentPatternPositions(version) {
  if (version === 1) {
    return [];
  }

  const numAlign = Math.floor(version / 7) + 2;
  const step = version === 32 ? 26 : Math.ceil((version * 4 + 4) / (numAlign * 2 - 2)) * 2;
  const result = [6];

  for (let pos = version * 4 + 10; result.length < numAlign; pos -= step) {
    result.splice(1, 0, pos);
  }

  return result;
}

function getMaskBit(mask, x, y) {
  switch (mask) {
    case 0:
      return (x + y) % 2 === 0;
    case 1:
      return y % 2 === 0;
    case 2:
      return x % 3 === 0;
    case 3:
      return (x + y) % 3 === 0;
    case 4:
      return (Math.floor(x / 3) + Math.floor(y / 2)) % 2 === 0;
    case 5:
      return ((x * y) % 2) + ((x * y) % 3) === 0;
    case 6:
      return (((x * y) % 2) + ((x * y) % 3)) % 2 === 0;
    case 7:
      return (((x + y) % 2) + ((x * y) % 3)) % 2 === 0;
    default:
      throw new Error("Invalid QR mask");
  }
}

function getPenaltyScore(modules) {
  const size = modules.length;
  let result = 0;

  for (let y = 0; y < size; y += 1) {
    result += linePenalty(modules[y]);
  }
  for (let x = 0; x < size; x += 1) {
    result += linePenalty(modules.map((row) => row[x]));
  }

  for (let y = 0; y < size - 1; y += 1) {
    for (let x = 0; x < size - 1; x += 1) {
      const color = modules[y][x];
      if (color === modules[y][x + 1] && color === modules[y + 1][x] && color === modules[y + 1][x + 1]) {
        result += 3;
      }
    }
  }

  for (let y = 0; y < size; y += 1) {
    result += finderPenalty(modules[y]);
  }
  for (let x = 0; x < size; x += 1) {
    result += finderPenalty(modules.map((row) => row[x]));
  }

  const dark = modules.flat().filter(Boolean).length;
  const total = size * size;
  result += Math.floor(Math.abs((dark * 100) / total - 50) / 5) * 10;

  return result;
}

function linePenalty(line) {
  let result = 0;
  let runColor = line[0];
  let runLength = 1;

  for (let i = 1; i < line.length; i += 1) {
    if (line[i] === runColor) {
      runLength += 1;
      if (runLength === 5) {
        result += 3;
      } else if (runLength > 5) {
        result += 1;
      }
    } else {
      runColor = line[i];
      runLength = 1;
    }
  }

  return result;
}

function finderPenalty(line) {
  let result = 0;
  const pattern = [true, false, true, true, true, false, true];

  for (let i = 0; i <= line.length - 7; i += 1) {
    if (!pattern.every((value, offset) => line[i + offset] === value)) {
      continue;
    }

    const before = i >= 4 && line.slice(i - 4, i).every((value) => !value);
    const after = i + 11 <= line.length && line.slice(i + 7, i + 11).every((value) => !value);
    if (before || after) {
      result += 40;
    }
  }

  return result;
}

function makeMatrix(size, value) {
  return Array.from({ length: size }, () => Array(size).fill(value));
}

function cloneMatrix(matrix) {
  return matrix.map((row) => row.slice());
}

function getBit(value, index) {
  return ((value >>> index) & 1) !== 0;
}

const ECC_CODEWORDS_PER_BLOCK = [
  0,
  10,
  16,
  26,
  18,
  24,
  16,
  18,
  22,
  22,
  26,
  30,
  22,
  22,
  24,
  24,
  28,
  28,
  26,
  26,
  26,
  28,
  28,
  28,
  28,
  28,
  28,
  28,
  28,
  28,
  28,
  28,
  28,
  28,
  28,
  28,
  28,
  28,
  28,
  28,
  28,
];

const NUM_ERROR_CORRECTION_BLOCKS = [
  0,
  1,
  1,
  1,
  2,
  2,
  4,
  4,
  4,
  5,
  5,
  5,
  8,
  9,
  9,
  10,
  10,
  11,
  13,
  14,
  16,
  17,
  17,
  18,
  20,
  21,
  23,
  25,
  26,
  28,
  29,
  31,
  33,
  35,
  37,
  38,
  40,
  43,
  45,
  47,
  49,
];

render();
