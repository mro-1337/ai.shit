load("sbbsdefs.js");

// ==========================================================================
// CONFIGURATION SECTION
// ==========================================================================
var HTML_PATH_1  = "/sbbs/web/root/"; 
var HTML_PATH_2  = "c:/web/"; 
var USER_DATA    = system.data_dir + "user/" + format("%04d", user.number) + ".weather.json";
var TMP_GEO      = system.temp_dir + "geo_data.json";
var TMP_WEATHER  = system.temp_dir + "weather_data.json";

// --- BBS COLORS ---
var COL_TITLE    = "\1h\1w"; 
var COL_ICON     = "\1h\1y"; 
var COL_CUR_VAL  = "\1n\1w"; 
var COL_HEAD_LBL = "\1h\1w"; 
var COL_BORDER   = "\1h\1k\1k"; // This is the color for dividers
var COL_DATE     = "\1h\1w"; 
var COL_COND     = "\1h\1w"; 
var COL_TEMP     = "\1h\1y"; 
// ==========================================================================

function getAscii(code) {
    var icons = {
        clear:   ["  \\   /  ", "   .-.   ", "--(   )--", "   `-`   ", "  /   \\  "],
        cloudy:  ["         ", "    .--. ", " .-(    )", "(___.__) ", "         "],
        rain:    ["    .--. ", " .-(    )", "(___.__) ", "  ' ' ' '", " ' ' ' ' "],
        snow:    ["    .--. ", " .-(    )", "(___.__) ", "  * * * ", " * * * * "],
        thunder: ["    .--. ", " .-(    )", "(___.__) ", "  /_  /_ ", "   /   / "]
    };
    if (code == 0 || code == 1) return icons.clear;
    if (code >= 2 && code <= 48) return icons.cloudy;
    if (code >= 51 && code <= 67 || code >= 80 && code <= 82) return icons.rain;
    if (code >= 71 && code <= 77 || code >= 85 && code <= 86) return icons.snow;
    if (code >= 95) return icons.thunder;
    return icons.cloudy;
}

function sbbsToHex(code) {
    var map = {"\1n\1k":"#000000","\1h\1k":"#555555","\1n\1r":"#aa0000","\1h\1r":"#ff5555","\1n\1g":"#00aa00","\1h\1g":"#55ff55","\1n\1y":"#aaaa00","\1h\1y":"#ffff55","\1n\1b":"#0000aa","\1h\1b":"#5555ff","\1n\1m":"#aa00aa","\1h\1m":"#ff55ff","\1n\1c":"#00aaaa","\1h\1c":"#55ffff","\1n\1w":"#aaaaaa","\1h\1w":"#ffffff"};
    return map[code] || "#aaaaaa";
}

function getCommandOutput(cmd, tmpFile) {
    system.exec(cmd);
    var f = new File(tmpFile);
    var data = null;
    if (f.open("r")) {
        var raw = f.readAll().join("");
        try { data = JSON.parse(raw); } catch(e) { log(LOG_ERR, "JSON Error: " + e); }
        f.close();
        file_remove(tmpFile);
    }
    return data;
}

function getCondition(code) {
    var desc = {0:"Clear",1:"Sunny",2:"P.Cloudy",3:"Overcast",45:"Foggy",51:"Drizzle",61:"Rain",71:"Snow",95:"T-Storm"};
    return desc[code] || "Cloudy";
}

// --- 1. Location Logic ---
var lat, lon, city, source = "IP Geolocation";
var forceManual = (argv.length > 0);

if (!forceManual && file_exists(USER_DATA)) {
    var f = new File(USER_DATA);
    if (f.open("r")) {
        try { var saved = JSON.parse(f.readAll().join("")); lat = saved.lat; lon = saved.lon; city = saved.city; source = "Home"; } catch(e){}
        f.close();
    }
}

if (!lat && !forceManual) {
    var geoData = getCommandOutput('curl -s "http://ip-api.com/json/' + client.ip_address + '" -o ' + TMP_GEO, TMP_GEO);
    if (geoData && geoData.status == "success") { lat = geoData.lat; lon = geoData.lon; city = geoData.city; }
}

if (forceManual || !lat) {
    console.clear();
    console.putmsg("\1h\1yManual Search\r\n\1n\1wCity: ");
    var searchCity = console.getstr(30, K_UPPER);
    var searchData = getCommandOutput('curl -s "https://geocoding-api.open-meteo.com/v1/search?name=' + searchCity.replace(/ /g, "+") + '&count=1" -o ' + TMP_GEO, TMP_GEO);
    if (searchData && searchData.results) {
        lat = searchData.results[0].latitude; lon = searchData.results[0].longitude; city = searchData.results[0].name;
        var f = new File(USER_DATA); if (f.open("w")) { f.write(JSON.stringify({lat:lat, lon:lon, city:city})); f.close(); }
        source = "Manual";
    } else { lat = "40.71"; lon = "-74.01"; city = "New York"; source = "Default"; }
}

// --- 2. Fetch Weather ---
var weatherJson = getCommandOutput('curl -s -L "https://api.open-meteo.com/v1/forecast?latitude='+lat+'&longitude='+lon+'&current_weather=true&daily=weathercode,temperature_2m_max,temperature_2m_min&temperature_unit=fahrenheit&timezone=auto" -o ' + TMP_WEATHER, TMP_WEATHER);

// --- 3. Build BBS Display ---
console.clear();
console.putmsg(COL_TITLE + "Weather report: " + city.toUpperCase() + " (" + source + ")\r\n\r\n");

var current = weatherJson.current_weather;
var iconLines = getAscii(current.weathercode);
var condStr = getCondition(current.weathercode);
var degSym = "F"; 

console.putmsg(COL_ICON + iconLines[0] + "  " + COL_CUR_VAL + condStr + "\r\n");
console.putmsg(COL_ICON + iconLines[1] + "  " + COL_CUR_VAL + Math.round(current.temperature) + " " + degSym + "\r\n");
console.putmsg(COL_ICON + iconLines[2] + "  " + COL_CUR_VAL + "Wind: " + Math.round(current.windspeed) + " mph\r\n");
console.putmsg(COL_ICON + iconLines[3] + "  \r\n");
console.putmsg(COL_ICON + iconLines[4] + "  \r\n\r\n");

console.putmsg(COL_HEAD_LBL + format("%-10s", "Date"));
console.putmsg(COL_BORDER   + " | ");
console.putmsg(COL_HEAD_LBL + format("%-13s", "Condition"));
console.putmsg(COL_BORDER   + " | ");
console.putmsg(COL_HEAD_LBL + "High / Low\r\n");
console.putmsg(COL_BORDER   + "-----------|---------------|-----------\r\n");

var daily = weatherJson.daily;
var forecastData = [];
for (var i = 0; i < 7; i++) {
    var dVal = daily.time[i].substring(5);
    var cVal = getCondition(daily.weathercode[i]);
    var tVal = Math.round(daily.temperature_2m_max[i]) + " / " + Math.round(daily.temperature_2m_min[i]);

    console.putmsg(COL_DATE + format("%-10s", dVal));
    console.putmsg(COL_BORDER + " \1h\1k| ");
    console.putmsg(COL_COND + format("%-13s", cVal));
    console.putmsg(COL_BORDER + " \1h\1k| ");
    console.putmsg(COL_TEMP + format("%7s", tVal) + "\r\n");

    forecastData.push({date: dVal, cond: cVal, temp: tVal});
}

var now = new Date();
var timeStr = now.toLocaleTimeString() + " " + now.toLocaleDateString();
console.putmsg("\r\n\1h\1kGenerated: " + timeStr + "\1n\r\n");

console.putmsg("\1n\1wPress any key...");
console.pause();

// --- 4. Generate HTML ---
var htmlContent = "<!DOCTYPE html><html><head>\n" +
    "<meta charset='UTF-8'>\n" +
    "<meta http-equiv='refresh' content='900'>\n" +
    "<meta http-equiv='Cache-Control' content='no-cache, no-store, must-revalidate'>\n" +
    "<meta http-equiv='Pragma' content='no-cache'>\n" +
    "<meta http-equiv='Expires' content='0'>\n" +
    "<style>\n" +
    "body { background-color: black; font-family: 'Courier New', monospace; line-height: 1.1; color: #aaaaaa; }\n" +
    ".icon  { color: " + sbbsToHex(COL_ICON) + "; }\n" +
    ".val   { color: " + sbbsToHex(COL_CUR_VAL) + "; font-weight: bold; }\n" +
    ".title { color: " + sbbsToHex(COL_TITLE) + "; font-size: 1.2em; }\n" +
    ".head  { color: " + sbbsToHex(COL_HEAD_LBL) + "; }\n" +
    ".border{ color: " + sbbsToHex(COL_BORDER) + "; }\n" +
    ".date  { color: " + sbbsToHex(COL_DATE) + "; }\n" +
    ".cond  { color: " + sbbsToHex(COL_COND) + "; }\n" +
    ".temp  { color: " + sbbsToHex(COL_TEMP) + "; }\n" +
    ".footer{ color: #555555; font-size: 0.8em; }\n" +
    "</style></head><body><pre>\n" +
    "<span class='title'>Weather report: " + city.toUpperCase() + "</span>\n\n" +
    "<span class='icon'>" + iconLines[0] + "</span>  <span class='val'>" + condStr + "</span>\n" +
    "<span class='icon'>" + iconLines[1] + "</span>  <span class='val'>" + Math.round(current.temperature) + " F</span>\n" +
    "<span class='icon'>" + iconLines[2] + "</span>  <span class='val'>Wind: " + Math.round(current.windspeed) + " mph</span>\n" +
    "<span class='icon'>" + iconLines[3] + "</span>\n" +
    "<span class='icon'>" + iconLines[4] + "</span>\n\n" +
    // FIX: Header dividers isolated and set to border class
    "<span class='head'>Date      </span><span class='border'> | </span>" +
    "<span class='head'>Condition    </span><span class='border'> | </span>" +
    "<span class='head'>High / Low</span>\n" +
    "<span class='border'>-----------|---------------|-----------</span>\n";

for (var j in forecastData) {
    var fd = forecastData[j];
    // FIX: Row dividers isolated and set to border class
    htmlContent += "<span class='date'>" + format("%-10s", fd.date) + "</span>" + 
                   "<span class='border'> | </span>" +
                   "<span class='cond'>" + format("%-13s", fd.cond) + "</span>" + 
                   "<span class='border'> | </span>" +
                   "<span class='temp'>" + format("%7s", fd.temp) + "</span>\n";
}

htmlContent += "\n<span class='footer'>Last Generated: " + timeStr + "</span>\n";
htmlContent += "</pre></body></html>";

function saveToPath(path) {
    var cleanPath = path.replace(/\//g, "\\");
    if (cleanPath.charAt(cleanPath.length - 1) !== "\\") {
        cleanPath += "\\";
    }
    var fileName = cleanPath + user.number + ".weather.html";
    var h = new File(fileName);
    if (h.open("w")) {
        h.write(htmlContent);
        h.close();
//        console.putmsg("\1n\1g[Web] Created: \1w" + fileName + "\r\n");
    } else {
        console.putmsg("\1h\1r[Web Error] Failed to write: \1w" + fileName + "\r\n");
    }
}

saveToPath(HTML_PATH_1);
saveToPath(HTML_PATH_2);