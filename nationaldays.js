// =============================================================================
// nationaldays.js  —  FIXED & IMPROVED (March 2026)
// Works with the current daysoftheyear.com structure
// =============================================================================

load("sbbsdefs.js");

// ========================== CONFIGURATION ==========================
var cfg = {
    max_show: 15,                    // 0 = show ALL

    color_header: "\1c\1h",          // bright cyan
    color_date:   "\1w\1h",          // bright white
    color_number: "\1g\1h",          // bright green
    color_text:   "\1y",             // yellow
    color_more:   "\1n\1c",          // normal cyan
    color_reset:  "\1n"
};

// ===================================================================
function fetch_page() {
    var today = strftime("%Y-%m-%d");
    var cache_file = system.temp_dir + "nationaldays_cache_" + today + ".html";

    if (file_exists(cache_file)) {
        var f = new File(cache_file);
        if (f.open("r")) {
            var html = f.readAll().join("\r\n");
            f.close();
            return html;
        }
    }

    var temp_file = system.temp_dir + "nationaldays_temp_" + today + ".html";

    var cmd = 'curl -s --max-time 30 -L -k -f ' +
              '-H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64)" ' +
              'https://www.daysoftheyear.com/today/ -o ' + temp_file;

    var exitcode = system.exec(cmd);

    if (exitcode !== 0 || !file_exists(temp_file)) {
        return "";
    }

    var f = new File(temp_file);
    if (f.open("r")) {
        var html = f.readAll().join("\r\n");
        f.close();

        var cf = new File(cache_file);
        if (cf.open("w")) {
            cf.write(html);
            cf.close();
        }
        file_remove(temp_file);
        return html;
    }
    return "";
}

function get_todays_holidays() {
    var html = fetch_page();
    if (html === "") return [];

    var days = [];

    // IMPROVED REGEX: Matches both <h2> and <h3> with links to /days/
    // This catches "Earth Hour" (h2) and all the regular days (h3)
    var heading_pattern = /<h[23][^>]*>[\s\S]*?<a[^>]*href="[^"]*\/days\/[^"]*"[^>]*>([^<]+)<\/a>[\s\S]*?<\/h[23]>/gi;

    var match;
    while ((match = heading_pattern.exec(html)) !== null) {
        var name = match[1].trim();
        name = name.replace(/&amp;/g, "&")
                   .replace(/&lt;/g, "<")
                   .replace(/&gt;/g, ">")
                   .replace(/&#039;/g, "'")
                   .replace(/&quot;/g, '"')
                   .replace(/&nbsp;/g, " ");

        // Filter to keep only things that sound like "National/International ... Day"
        if (/\b(Day|Week|Month|Hour|Night|Awareness)s?$/i.test(name)) {
            if (name.length > 5 && name.length < 120 && days.indexOf(name) === -1) {
                days.push(name);
            }
        }
    }

    if (days.length > 0) {
        return days;
    }

    // Fallback text-based method (rarely needed now)
    var text = html.replace(/<[^>]+>/g, " ")
                   .replace(/\s+/g, " ");

    var fallback_pattern = /\b(?:National|International|World|Global)\s+[^.]{5,80}?(?:Day|Week|Month|Hour|Night|Awareness)\b/gi;
    var fallback_match;
    while ((fallback_match = fallback_pattern.exec(text)) !== null) {
        var name = fallback_match[0].trim();
        if (days.indexOf(name) === -1) {
            days.push(name);
        }
    }

    return days;
}

// ========================== MAIN ==========================
console.clear();

console.print("\r\n");
console.print(cfg.color_header + "\xDB\xDB\xDB TODAY'S NATIONAL DAYS \xDB\xDB\xDB" + cfg.color_reset + "\r\n");
console.print(cfg.color_date + strftime("%A, %B %d, %Y") + cfg.color_reset + "\r\n\r\n");

var days = get_todays_holidays();

if (days.length === 0) {
    console.print("   National days service temporarily unavailable today.\r\n");
} else {
    var show = (cfg.max_show === 0) ? days.length : Math.min(cfg.max_show, days.length);

    for (var i = 0; i < show; i++) {
        console.print(cfg.color_number + (i+1) + ". " +
                      cfg.color_text + days[i] + cfg.color_reset + "\r\n");
    }

    if (show < days.length) {
        console.print(cfg.color_more + "   ... and " + (days.length - show) + " more!\r\n");
    }
}

console.print(cfg.color_reset + "\r\n");