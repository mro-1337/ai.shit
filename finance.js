// Synchronet BBS Finance - Final Stable Version
// Features: 60x15 Resolution, Month Labels, Stock/Crypto Search
load("http.js");

// Safety: Define characters to prevent "unterminated string" errors
var B_BOT_LEFT = String.fromCharCode(192); // └
var B_HORIZ    = String.fromCharCode(196); // ─
var B_VERT     = String.fromCharCode(179); // │
var B_BLOCK    = String.fromCharCode(177); // ▒
var B_HALF     = String.fromCharCode(223); // ▀

function getHistory(symbol, isCrypto) {
    var req = new HTTPRequest();
    var url = isCrypto 
        ? "https://api.coingecko.com/api/v3/coins/" + 
          ({"BTC":"bitcoin","ETH":"ethereum","SOL":"solana","XRP":"ripple","BNB":"binancecoin"}[symbol] || symbol.toLowerCase()) + 
          "/market_chart?vs_currency=usd&days=180&interval=daily"
        : "https://query1.finance.yahoo.com/v8/finance/chart/" + symbol + "?range=6mo&interval=1d";

    try {
        var res = req.Get(url);
        if (!res) return null;
        var data = JSON.parse(res);
        var prices = [];
        if (isCrypto && data.prices) {
            for (var i = 0; i < data.prices.length; i++) prices.push(Number(data.prices[i][1]));
        } else if (!isCrypto && data.chart.result[0].indicators.quote[0].close) {
            var d = data.chart.result[0].indicators.quote[0].close;
            for(var j=0; j<d.length; j++) if(d[j] != null) prices.push(Number(d[j]));
        }
        return (prices.length > 0) ? prices : null;
    } catch (e) { return null; }
}

function getMonthsLabels() {
    var mNames = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
    var d = new Date();
    var out = "           ";
    for (var i = 5; i >= 0; i--) {
        var mIdx = (d.getMonth() - i);
        if (mIdx < 0) mIdx += 12;
        var s = mNames[mIdx];
        while (s.length < 10) s += " ";
        out += s;
    }
    return "\x01n\x01w" + out;
}

function drawGraph(symbol, history) {
    var gW = 60, gH = 15;
    if (!history) {
        console.putmsg("\x01n\x01c" + symbol + ": \x01h\x01rData Unavailable\r\n\r\n");
        return;
    }
    var cur = Number(history[history.length - 1]);
    var start = Number(history[0]);
    var pct = ((cur - start) / start) * 100;
    var min = Math.min.apply(null, history), max = Math.max.apply(null, history);
    var range = max - min;

    console.putmsg("\x01n\x01c" + symbol + ": \x01h\x01w$" + cur.toFixed(2) + " ");
    console.putmsg((pct >= 0 ? "\x01h\x01g+" : "\x01h\x01r") + pct.toFixed(2) + "% (6mo)\r\n");

    for (var y = gH; y > 0; y--) {
        var lbl = (y === gH ? max.toFixed(0) : y === 1 ? min.toFixed(0) : "");
        while (lbl.length < 10) lbl = " " + lbl;
        var row = "\x01h\x01k" + lbl + " " + B_VERT;
        for (var x = 0; x < gW; x++) {
            var val = history[Math.floor((x / gW) * history.length)];
            var plot = ((val - min) / (range || 1)) * gH;
            if (plot >= y) row += "\x01n\x01c" + B_BLOCK;
            else if (plot >= y - 0.5) row += "\x01h\x01c" + B_HALF;
            else row += " ";
        }
        console.putmsg(row + "\r\n");
    }
    var bdr = ""; for(var i=0; i<gW; i++) bdr += B_HORIZ;
    console.putmsg("\x01h\x01k           " + B_BOT_LEFT + bdr + "\r\n");
    console.putmsg(getMonthsLabels() + "\r\n\r\n");
}

function main() {
    var stocks = ["NVDA", "AAPL", "GOOGL", "MSFT", "AMZN"];
    var cryptos = ["BTC", "ETH", "SOL", "XRP", "BNB"];
    console.clear();
    
    console.putmsg("\x01h\x01y--- TOP 5 STOCKS ---\x01n\r\n\r\n");
    for (var i = 0; i < stocks.length; i++) {
        drawGraph(stocks[i], getHistory(stocks[i], false));
        console.putmsg("\x01c[Next]"); console.getkey(); console.clear();
    }

    console.putmsg("\x01h\x01y--- TOP 5 CRYPTO ---\x01n\r\n\r\n");
    for (var j = 0; j < cryptos.length; j++) {
        drawGraph(cryptos[j], getHistory(cryptos[j], true));
        console.putmsg("\x01c[Next]"); console.getkey(); console.clear();
    }

    while (true) {
        console.putmsg("\x01h\x01wSearch Symbol (or ENTER to exit): \x01n");
        var s = console.getstr(12).toUpperCase();
        if (!s) break;
        console.clear();
        var h = getHistory(s, false) || getHistory(s, true);
        drawGraph(s, h);
    }
}
main();