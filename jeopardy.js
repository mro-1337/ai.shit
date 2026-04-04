// =============================================================================
//  JEOPARDY! for Synchronet BBS  v2.1
//  CP437 ANSI Graphics - Fully Configurable
//  Install in: sbbs/xtrn/jeopardy/
//  Entry: jeopardy.js
//  Compatible with Synchronet SpiderMonkey (JS1.5+)
// =============================================================================

// Load Synchronet BBS definitions (K_* constants, etc.)
try { load("sbbsdefs.js"); } catch(e) {}

// Polyfills for older SpiderMonkey in Synchronet
if (typeof Array.isArray === "undefined") {
    Array.isArray = function(v) {
        return Object.prototype.toString.call(v) === "[object Array]";
    };
}
if (typeof Date.now === "undefined") {
    Date.now = function() { return new Date().getTime(); };
}
if (typeof String.prototype.trim === "undefined") {
    String.prototype.trim = function() {
        return this.replace(/^\s+|\s+$/g, "");
    };
}

// Key mode constants (fallback if sbbsdefs.js not loaded)
var K_NONE   = 0;
var K_UPPER  = 1;
var K_NOECHO = 2;
var K_ECHO   = 4;

var VERSION  = "2.1";
var GAME_DIR = (typeof js !== "undefined" && js.exec_dir) ? js.exec_dir : "./";

// External questions loaded from file (kept separate for the editor)
var EXT_ROUNDS = [];

// =============================================================================
//  DEFAULT CONFIGURATION  (overridden by jeopardy.ini)
// =============================================================================
var CFG = {
    title:           "JEOPARDY!",
    subtitle:        "The BBS Edition",
    values:          [200, 400, 600, 800, 1000],
    num_categories:  6,
    time_limit:      30,            // seconds per clue
    daily_doubles:   2,             // number of daily doubles per round
    allow_negative:  true,          // allow score to go below zero
    partial_match:   true,          // accept partial answers
    max_scores:      15,
    max_players:     4,       // maximum players per game (hot-seat)
    score_file:      GAME_DIR + "jeopardy.scr",
    questions_file:  GAME_DIR + "questions.json",

    // --- Hint / answer reveal mode ---
    // 0 = none (default)  1 = underscores only  2 = first letter + underscores
    hint_mode:       0,

    // --- Question source ---
    use_builtin:     true,   // set false to disable the 5 built-in rounds

    // --- Sysop access ---
    sysop_level:     99,     // minimum user security level for the sysop menu

    // --- Color indices (ANSI: 0=Blk 1=Red 2=Grn 3=Yel 4=Blu 5=Mag 6=Cyn 7=Wht) ---
    c_board_bg:      4,   // Board background        (blue)
    c_cat_fg:        7,   // Category name text      (white)
    c_val_fg:        3,   // Dollar amount text      (yellow)
    c_used_fg:       4,   // Used clue (hidden)      (blue on blue)
    c_sel_fg:        0,   // Selected cell fg        (black)
    c_sel_bg:        3,   // Selected cell bg        (yellow)
    c_border_fg:     7,   // Border characters       (white)
    c_title_fg:      3,   // Title text              (yellow)
    c_clue_fg:       7,   // Clue text               (bright white)
    c_correct_bg:    2,   // Correct answer bar      (green)
    c_wrong_bg:      1,   // Wrong answer bar        (red)
    c_score_fg:      3,   // Score display           (yellow)
    c_dd_fg:         5,   // Daily double color      (magenta)
    c_timer_ok:      2,   // Timer > 10s             (green)
    c_timer_warn:    3,   // Timer 5-10s             (yellow)
    c_timer_low:     1,   // Timer < 5s              (red)
};

// =============================================================================
//  LOAD INI CONFIGURATION
// =============================================================================
function load_config() {
    var f = new File(GAME_DIR + "jeopardy.ini");
    if (!f.open("r")) return;

    var g_keys = ["title","subtitle","time_limit","daily_doubles",
                  "allow_negative","partial_match","max_scores","max_players",
                  "hint_mode","use_builtin","sysop_level"];
    for (var i = 0; i < g_keys.length; i++) {
        var v = f.iniGetValue("game", g_keys[i], null);
        if (v !== null && v !== undefined) CFG[g_keys[i]] = String(v);
    }
    var sf = f.iniGetValue("files", "scores", null);
    if (sf) {
        sf = String(sf);
        CFG.score_file = (sf.indexOf("/") < 0 && sf.indexOf("\\") < 0) ? GAME_DIR + sf : sf;
    }
    var qf = f.iniGetValue("files", "questions", null);
    if (qf) {
        qf = String(qf);
        CFG.questions_file = (qf.indexOf("/") < 0 && qf.indexOf("\\") < 0) ? GAME_DIR + qf : qf;
    }

    // Color overrides
    var c_keys = ["c_board_bg","c_cat_fg","c_val_fg","c_used_fg","c_sel_fg",
                  "c_sel_bg","c_border_fg","c_title_fg","c_clue_fg",
                  "c_correct_bg","c_wrong_bg","c_score_fg","c_dd_fg",
                  "c_timer_ok","c_timer_warn","c_timer_low"];
    for (var ci = 0; ci < c_keys.length; ci++) {
        var cv = f.iniGetValue("colors", c_keys[ci], null);
        if (cv !== null) CFG[c_keys[ci]] = parseInt(cv);
    }

    // Parse value array
    // NOTE: Synchronet's iniGetValue may return an Array if the value contains commas
    var vals = f.iniGetValue("game", "values", null);
    if (vals !== null && vals !== undefined) {
        var vals_str = Array.isArray(vals) ? vals.join(",") : String(vals);
        var parts = vals_str.split(",");
        var arr = [];
        for (var vi = 0; vi < parts.length; vi++) {
            var n = parseInt(String(parts[vi]).trim());
            if (!isNaN(n)) arr.push(n);
        }
        if (arr.length > 0) CFG.values = arr;
    }

    // Type coerce
    CFG.time_limit      = parseInt(CFG.time_limit);
    CFG.daily_doubles   = parseInt(CFG.daily_doubles);
    CFG.max_scores      = parseInt(CFG.max_scores);
    CFG.max_players     = parseInt(CFG.max_players);
    CFG.hint_mode       = parseInt(CFG.hint_mode);
    CFG.sysop_level     = parseInt(CFG.sysop_level);
    CFG.allow_negative  = (CFG.allow_negative === true || CFG.allow_negative === "true");
    CFG.partial_match   = (CFG.partial_match  === true || CFG.partial_match  === "true");
    CFG.use_builtin     = (CFG.use_builtin    === true || CFG.use_builtin    === "true");

    f.close();
}

// =============================================================================
//  SAVE INI CONFIGURATION  (called by sysop settings editor)
// =============================================================================
function save_config() {
    var f = new File(GAME_DIR + "jeopardy.ini");
    if (!f.open("w")) return false;

    f.writeln("; Jeopardy! BBS Edition - jeopardy.ini  (auto-saved by sysop menu)");
    f.writeln("");
    f.writeln("[game]");
    f.writeln("title="        + CFG.title);
    f.writeln("subtitle="     + CFG.subtitle);
    f.writeln("values="       + CFG.values.join(","));
    f.writeln("time_limit="   + CFG.time_limit);
    f.writeln("daily_doubles="+ CFG.daily_doubles);
    f.writeln("allow_negative="+ (CFG.allow_negative ? "true" : "false"));
    f.writeln("partial_match=" + (CFG.partial_match  ? "true" : "false"));
    f.writeln("max_scores="   + CFG.max_scores);
    f.writeln("max_players="  + CFG.max_players);
    f.writeln("hint_mode="    + CFG.hint_mode);
    f.writeln("use_builtin="  + (CFG.use_builtin ? "true" : "false"));
    f.writeln("sysop_level="  + CFG.sysop_level);
    f.writeln("");
    f.writeln("[files]");
    f.writeln("scores="    + CFG.score_file);
    f.writeln("questions=" + CFG.questions_file);
    f.writeln("");
    f.writeln("[colors]");
    var c_keys = ["c_board_bg","c_cat_fg","c_val_fg","c_used_fg","c_sel_fg",
                  "c_sel_bg","c_border_fg","c_title_fg","c_clue_fg",
                  "c_correct_bg","c_wrong_bg","c_score_fg","c_dd_fg",
                  "c_timer_ok","c_timer_warn","c_timer_low"];
    for (var ci = 0; ci < c_keys.length; ci++) {
        f.writeln(c_keys[ci] + "=" + CFG[c_keys[ci]]);
    }
    f.close();
    return true;
}

// =============================================================================
//  ANSI / CP437 UTILITY LAYER
// =============================================================================
var ESC   = "\x1b[";
var RESET = "\x1b[0m";

// Build ANSI color sequence
// fg/bg: 0-7 color index, bright: boolean
function clr(fg, bg, bright) {
    var s = "\x1b[" + (bright ? "1" : "0");
    if (fg !== null && fg !== undefined) s += ";" + (30 + fg);
    if (bg !== null && bg !== undefined) s += ";" + (40 + bg);
    return s + "m";
}

function gotoxy(x, y)   { console.write("\x1b[" + y + ";" + x + "H"); }
function hide_cursor()  { console.write("\x1b[?25l"); }
function show_cursor()  { console.write("\x1b[?25h"); }
function clrscr()       { console.write("\x1b[2J\x1b[1;1H"); }
function save_pos()     { console.write("\x1b[s"); }
function restore_pos()  { console.write("\x1b[u"); }

function rep(ch, n) {
    var s = "";
    for (var i = 0; i < n; i++) s += ch;
    return s;
}

function pad_center(str, w) {
    str = String(str);
    if (str.length >= w) return str.substring(0, w);
    var p = w - str.length;
    return rep(" ", Math.floor(p / 2)) + str + rep(" ", Math.ceil(p / 2));
}

function pad_right(str, w) {
    str = String(str);
    if (str.length >= w) return str.substring(0, w);
    return str + rep(" ", w - str.length);
}

function pad_left(str, w) {
    str = String(str);
    if (str.length >= w) return str.substring(0, w);
    return rep(" ", w - str.length) + str;
}

function word_wrap(str, width) {
    var words = str.split(" ");
    var lines = [];
    var line  = "";
    for (var i = 0; i < words.length; i++) {
        if (line === "") {
            line = words[i];
        } else if (line.length + 1 + words[i].length <= width) {
            line += " " + words[i];
        } else {
            lines.push(line);
            line = words[i];
        }
    }
    if (line) lines.push(line);
    return lines;
}

function format_money(n) {
    var abs  = Math.abs(n);
    var sign = (n < 0) ? "-$" : "$";
    var s = String(abs);
    var out = "";
    var mod = s.length % 3;
    for (var i = 0; i < s.length; i++) {
        if (i > 0 && (i % 3 === mod)) out += ",";
        out += s[i];
    }
    return sign + out;
}

// Write a complete 80-column line: left margin + box row + right margin.
// bx       = leftmost column of box (1-based)
// bw       = total box width including both border chars
// bg       = background color index for the screen fill
// content  = pre-colored string that is exactly (bw-2) VISUAL chars wide
// fg       = border color index
function full_line(y, bx, bw, bg, fg, content) {
    var left_pad  = bx - 1;          // spaces before the left border
    var right_pad = 80 - bx - bw + 1; // spaces after the right border
    if (right_pad < 0) right_pad = 0;
    gotoxy(1, y);
    console.write(
        clr(7, bg, false) + rep(" ", left_pad) +
        clr(fg, bg, true) + B.V +
        content +
        clr(fg, bg, true) + B.V +
        clr(7, bg, false) + rep(" ", right_pad) +
        RESET
    );
}

// Write a full 80-column separator/border line
function full_sep(y, bx, bw, bg, fg, lc, mc, rc) {
    var left_pad  = bx - 1;
    var right_pad = 80 - bx - bw + 1;
    if (right_pad < 0) right_pad = 0;
    gotoxy(1, y);
    console.write(
        clr(7, bg, false) + rep(" ", left_pad) +
        clr(fg, bg, true) + lc + rep(B.H, bw - 2) + rc +
        clr(7, bg, false) + rep(" ", right_pad) +
        RESET
    );
}

// CP437 box drawing characters (double-line heavy)
var B = {
    TL:   "\xC9",   // ?
    TR:   "\xBB",   // ?
    BL:   "\xC8",   // ?
    BR:   "\xBC",   // ?
    H:    "\xCD",   // ?
    V:    "\xBA",   // ?
    TM:   "\xCB",   // ?
    BM:   "\xCA",   // ?
    ML:   "\xCC",   // ?
    MR:   "\xB9",   // ?
    XX:   "\xCE",   // ?
    // single-line
    sh:   "\xC4",   // ?
    sv:   "\xB3",   // ?
    stl:  "\xDA",   // ?
    str_: "\xBF",   // ?
    sbl:  "\xC0",   // ?
    sbr:  "\xD9",   // ?
    slt:  "\xC3",   // ?
    srt:  "\xB4",   // ?
    // block elements
    FULL: "\xDB",   // ?
    DARK: "\xB2",   // ?
    MED:  "\xB1",   // ?
    LITE: "\xB0",   // ?
    LOWER:"\xDC",   // ?
    UPPER:"\xDF",   // ?
    LEFT: "\xDD",   // ?
    RIGHT:"\xDE",   // ?
    // misc
    BULL: "\xF9",   // ?
    DIAM: "\x04",   // ?
    STAR: "\x0F",   // ?
};

// =============================================================================
//  BOARD LAYOUT CONSTANTS
// =============================================================================
// 80-column board: 6 cols x 12 wide + 7 borders = 79 (+1 left margin = 80)
var N_COLS   = 6;
var COL_W    = 12;
var BOARD_W  = N_COLS * COL_W + (N_COLS + 1);  // 79
var BOARD_X  = 1;
var BOARD_Y  = 1;

// Row positions within the 24-line screen
var ROW_TITLE   = 1;
var ROW_TOP     = 2;
var ROW_CAT1    = 3;
var ROW_CAT2    = 4;
var ROW_CATSEP  = 5;
// Value rows: 6, 8, 10, 12, 14  (seps at 7,9,11,13)
var ROW_VAL_BASE = 6;
var ROW_VAL_SEP  = 2;  // spacing between rows
var ROW_BOT;            // computed in draw_board
var ROW_SCORE;
var ROW_NAV;
var ROW_STATUS;

function calc_rows() {
    var n = CFG.values.length;
    ROW_BOT    = ROW_VAL_BASE + (n - 1) * ROW_VAL_SEP + 1;
    ROW_SCORE  = ROW_BOT + 1;
    ROW_NAV    = ROW_BOT + 2;
    ROW_STATUS = ROW_BOT + 3;
}

// =============================================================================
//  QUESTIONS DATABASE
// =============================================================================
// Format: array of rounds (each round = array of 6 category objects)
// Category: { name: "CATEGORY NAME", clues: [ {q: "clue text", a: "answer"}, ... ] }
// Clues indexed 0-4 matching CFG.values[0-4]

var QUESTION_ROUNDS = [

// ============================================================ ROUND 1
[
 { name: "SCIENCE",
   clues: [
    { q: "The planet known as the Red Planet", a: "Mars" },
    { q: "Chemical symbol for gold on the periodic table", a: "Au" },
    { q: "Gas making up ~78% of Earth's atmosphere", a: "Nitrogen" },
    { q: "The force that keeps planets in orbit around the Sun", a: "Gravity" },
    { q: "Scientist who developed the theory of general relativity", a: "Einstein" }
   ]},
 { name: "U.S. HISTORY",
   clues: [
    { q: "Year the Declaration of Independence was signed", a: "1776" },
    { q: "President who issued the Emancipation Proclamation", a: "Lincoln" },
    { q: "First state to ratify the U.S. Constitution", a: "Delaware" },
    { q: "The war fought from 1861 to 1865 on American soil", a: "Civil War" },
    { q: "Constitutional amendment that abolished slavery", a: "13th Amendment" }
   ]},
 { name: "GEOGRAPHY",
   clues: [
    { q: "The capital of France", a: "Paris" },
    { q: "The longest river in the world", a: "The Nile" },
    { q: "Continent containing the Amazon rainforest", a: "South America" },
    { q: "The smallest country in the world by area", a: "Vatican City" },
    { q: "Mountain range separating Europe from Asia", a: "The Urals" }
   ]},
 { name: "LITERATURE",
   clues: [
    { q: "Author of 'To Kill a Mockingbird'", a: "Harper Lee" },
    { q: "Shakespeare's play about a Danish prince", a: "Hamlet" },
    { q: "Russian author who wrote 'War and Peace'", a: "Tolstoy" },
    { q: "'The Great Gatsby' was written by this author", a: "F. Scott Fitzgerald" },
    { q: "George Orwell's dystopian novel set in the year 1984", a: "Nineteen Eighty-Four" }
   ]},
 { name: "POP CULTURE",
   clues: [
    { q: "Boy wizard who attends Hogwarts School", a: "Harry Potter" },
    { q: "Singer known as the 'Queen of Pop'", a: "Madonna" },
    { q: "This game features a plumber named Mario", a: "Super Mario Bros" },
    { q: "The TV show set in the fictional Westeros", a: "Game of Thrones" },
    { q: "Fictional detective at 221B Baker Street", a: "Sherlock Holmes" }
   ]},
 { name: "SPORTS",
   clues: [
    { q: "Sport played at Wimbledon", a: "Tennis" },
    { q: "Boxer known as 'The Greatest'", a: "Muhammad Ali" },
    { q: "Number of players on a basketball team on the court", a: "Five" },
    { q: "Country that has won the most FIFA World Cups", a: "Brazil" },
    { q: "Golfer with the most major championship wins", a: "Jack Nicklaus" }
   ]}
],

// ============================================================ ROUND 2
[
 { name: "ANIMALS",
   clues: [
    { q: "The largest land animal on Earth", a: "African elephant" },
    { q: "A group of lions is called this", a: "A pride" },
    { q: "The national bird of the United States", a: "Bald eagle" },
    { q: "The fastest land animal on Earth", a: "Cheetah" },
    { q: "This mammal lays eggs and has a duck-like bill", a: "Platypus" }
   ]},
 { name: "MATH",
   clues: [
    { q: "The square root of 144", a: "12" },
    { q: "Pi is approximately this value to 5 decimal places", a: "3.14159" },
    { q: "In the Fibonacci sequence 0,1,1,2,3... the next number is", a: "5" },
    { q: "Degrees in a right angle", a: "90" },
    { q: "Theorem stating a squared plus b squared equals c squared", a: "Pythagorean theorem" }
   ]},
 { name: "THE MOVIES",
   clues: [
    { q: "'I'll be back' is a famous line from this 1984 film", a: "The Terminator" },
    { q: "Director of 'Schindler's List' and 'Jurassic Park'", a: "Steven Spielberg" },
    { q: "The 1977 film that begins 'A long time ago in a galaxy far, far away'", a: "Star Wars" },
    { q: "Actress who played Scarlett O'Hara in 'Gone with the Wind'", a: "Vivien Leigh" },
    { q: "The first film to win 11 Academy Awards", a: "Ben-Hur" }
   ]},
 { name: "FOOD & DRINK",
   clues: [
    { q: "The main ingredient in guacamole", a: "Avocado" },
    { q: "This Japanese dish consists of vinegared rice with raw fish", a: "Sushi" },
    { q: "Italian cheese used in a Caprese salad", a: "Mozzarella" },
    { q: "The country most associated with the invention of croissants", a: "France" },
    { q: "Fermented grape juice served at dinner", a: "Wine" }
   ]},
 { name: "MUSIC",
   clues: [
    { q: "Band that released 'Abbey Road' in 1969", a: "The Beatles" },
    { q: "Guitarist nicknamed 'Slowhand'", a: "Eric Clapton" },
    { q: "Country of origin for the musical genre bossa nova", a: "Brazil" },
    { q: "Composer of 'The Four Seasons'", a: "Vivaldi" },
    { q: "Michael Jackson's 1982 landmark album title", a: "Thriller" }
   ]},
 { name: "PRESIDENTS",
   clues: [
    { q: "The first U.S. President", a: "George Washington" },
    { q: "President who signed the Louisiana Purchase", a: "Thomas Jefferson" },
    { q: "The only U.S. president to resign from office", a: "Richard Nixon" },
    { q: "President who served the shortest term in office", a: "William Henry Harrison" },
    { q: "FDR stood for Franklin this Roosevelt", a: "Delano" }
   ]}
],

// ============================================================ ROUND 3
[
 { name: "TECHNOLOGY",
   clues: [
    { q: "Company that created the iPhone", a: "Apple" },
    { q: "Programming language whose mascot is a snake", a: "Python" },
    { q: "Inventor of the World Wide Web", a: "Tim Berners-Lee" },
    { q: "This search engine's name means 10 to the 100th power", a: "Google" },
    { q: "The first IBM-compatible personal computer was released in this year", a: "1981" }
   ]},
 { name: "WORLD HISTORY",
   clues: [
    { q: "Year World War II ended", a: "1945" },
    { q: "Napoleon was exiled to this South Atlantic island", a: "St. Helena" },
    { q: "This empire was ruled by Genghis Khan", a: "Mongol Empire" },
    { q: "Ship that carried the Pilgrims to America in 1620", a: "The Mayflower" },
    { q: "The ancient wonder of the world located in Alexandria, Egypt", a: "The Lighthouse" }
   ]},
 { name: "ART",
   clues: [
    { q: "Leonardo da Vinci painted this famous smiling woman", a: "The Mona Lisa" },
    { q: "Spanish painter who created 'Guernica'", a: "Picasso" },
    { q: "Van Gogh cut off this body part", a: "His ear" },
    { q: "Art movement characterized by visible brushstrokes and capturing light", a: "Impressionism" },
    { q: "Sculptor who created 'The Thinker'", a: "Rodin" }
   ]},
 { name: "TELEVISION",
   clues: [
    { q: "NBC show set at the fictional Dunder Mifflin paper company", a: "The Office" },
    { q: "Walter White is the main character in this AMC drama", a: "Breaking Bad" },
    { q: "Long-running animated show featuring the Simpson family", a: "The Simpsons" },
    { q: "This show featured Leslie Knope in the town of Pawnee, Indiana", a: "Parks and Recreation" },
    { q: "Sci-fi show where crew of the Enterprise boldly goes where no one has gone before", a: "Star Trek" }
   ]},
 { name: "NATURE",
   clues: [
    { q: "Process by which plants make food using sunlight", a: "Photosynthesis" },
    { q: "Layer of the Earth between the crust and core", a: "The mantle" },
    { q: "Phenomenon that occurs when the moon blocks the sun", a: "Solar eclipse" },
    { q: "The world's largest ocean", a: "Pacific Ocean" },
    { q: "The geologic era known as the Age of Dinosaurs", a: "Mesozoic Era" }
   ]},
 { name: "FAMOUS PEOPLE",
   clues: [
    { q: "Civil rights leader who delivered the 'I Have a Dream' speech", a: "Martin Luther King Jr" },
    { q: "First woman to win a Nobel Prize", a: "Marie Curie" },
    { q: "This inventor held over 1,000 U.S. patents", a: "Thomas Edison" },
    { q: "First human to walk on the moon", a: "Neil Armstrong" },
    { q: "Greek philosopher who was the teacher of Plato", a: "Socrates" }
   ]}
],

// ============================================================ ROUND 4
[
 { name: "ASTRONOMY",
   clues: [
    { q: "The closest star to Earth other than the Sun", a: "Proxima Centauri" },
    { q: "This dwarf planet was reclassified in 2006", a: "Pluto" },
    { q: "The galaxy containing our solar system", a: "The Milky Way" },
    { q: "The theoretical point of no return around a black hole", a: "Event horizon" },
    { q: "Number of known moons orbiting Jupiter as of recent counts (approx.)", a: "95" }
   ]},
 { name: "AMERICAN CITIES",
   clues: [
    { q: "The Windy City", a: "Chicago" },
    { q: "City home to the Liberty Bell", a: "Philadelphia" },
    { q: "City nicknamed The Big Easy", a: "New Orleans" },
    { q: "The city called The Emerald City", a: "Seattle" },
    { q: "City where jazz was born in the early 20th century", a: "New Orleans" }
   ]},
 { name: "MYTHOLOGY",
   clues: [
    { q: "The Greek god of the sea", a: "Poseidon" },
    { q: "Norse god who wields the hammer Mjolnir", a: "Thor" },
    { q: "Achilles was only vulnerable in this body part", a: "His heel" },
    { q: "Roman god of war", a: "Mars" },
    { q: "In Egyptian mythology, the god who judged the dead", a: "Osiris" }
   ]},
 { name: "WORD PLAY",
   clues: [
    { q: "A word that reads the same forwards and backwards", a: "Palindrome" },
    { q: "A word that sounds like what it describes, like buzz or crash", a: "Onomatopoeia" },
    { q: "The figure of speech using 'like' or 'as' to compare things", a: "Simile" },
    { q: "When a part of something represents the whole, e.g. 'boots on the ground'", a: "Synecdoche" },
    { q: "A story in which characters represent abstract ideas or moral qualities", a: "Allegory" }
   ]},
 { name: "CHEMISTRY",
   clues: [
    { q: "The atomic number of carbon", a: "6" },
    { q: "The element with chemical symbol Fe", a: "Iron" },
    { q: "The most abundant element in the universe", a: "Hydrogen" },
    { q: "The element named after the Americas", a: "Americium" },
    { q: "This noble gas is used in colorful neon signs", a: "Neon" }
   ]},
 { name: "SPORTS RECORDS",
   clues: [
    { q: "NBA player who scored 100 points in a single game in 1962", a: "Wilt Chamberlain" },
    { q: "Country that has won the most total Olympic gold medals", a: "United States" },
    { q: "Swimmer who won 8 gold medals at the 2008 Beijing Olympics", a: "Michael Phelps" },
    { q: "Tennis player with the most Grand Slam singles titles (women)", a: "Serena Williams" },
    { q: "The only pitcher to throw a perfect game in the World Series", a: "Don Larsen" }
   ]}
],

// ============================================================ ROUND 5
[
 { name: "COMPUTERS",
   clues: [
    { q: "The operating system created by Linus Torvalds", a: "Linux" },
    { q: "Language used to create web page structure", a: "HTML" },
    { q: "Storage unit equal to 1,024 megabytes", a: "Gigabyte" },
    { q: "The company that makes the Windows operating system", a: "Microsoft" },
    { q: "This dial-up terminal network predated the modern internet", a: "BBS" }
   ]},
 { name: "THE 80s",
   clues: [
    { q: "The Rubik's Cube toy was invented in this decade (it became a craze)", a: "1980s" },
    { q: "Video game about a yellow circle eating dots and avoiding ghosts", a: "Pac-Man" },
    { q: "This 1984 film features a time-traveling DeLorean", a: "Back to the Future" },
    { q: "MTV launched in 1981 with this Buggles song", a: "Video Killed the Radio Star" },
    { q: "This band released 'Thriller' in 1982", a: "Michael Jackson" }
   ]},
 { name: "THE HUMAN BODY",
   clues: [
    { q: "The largest organ of the human body", a: "Skin" },
    { q: "The bone that protects the brain", a: "Skull" },
    { q: "Number of bones in the adult human body", a: "206" },
    { q: "The chamber of the heart that pumps blood to the body", a: "Left ventricle" },
    { q: "This gland in the neck regulates metabolism", a: "Thyroid" }
   ]},
 { name: "WORLD CAPITALS",
   clues: [
    { q: "Capital of Japan", a: "Tokyo" },
    { q: "Capital of Australia", a: "Canberra" },
    { q: "Capital of Canada", a: "Ottawa" },
    { q: "Capital of Brazil", a: "Brasilia" },
    { q: "Capital of Egypt", a: "Cairo" }
   ]},
 { name: "THE BIBLE",
   clues: [
    { q: "The first book of the Old Testament", a: "Genesis" },
    { q: "The man who built an ark to survive the great flood", a: "Noah" },
    { q: "This king of Israel was known for his great wisdom", a: "Solomon" },
    { q: "The last book of the New Testament", a: "Revelation" },
    { q: "The language in which most of the New Testament was originally written", a: "Greek" }
   ]},
 { name: "POTPOURRI",
   clues: [
    { q: "The number of sides on a hexagon", a: "Six" },
    { q: "Instrument with 88 keys", a: "Piano" },
    { q: "The process of boiling seawater to make it drinkable", a: "Desalination" },
    { q: "Country that gave the Statue of Liberty to the United States", a: "France" },
    { q: "The speed at which sound travels through air (approx) in mph", a: "767 mph" }
   ]}
]

]; // end QUESTION_ROUNDS

// Keep a static reference to the built-in rounds so rebuild_question_rounds()
// can restore them after the sysop toggles use_builtin on/off.
var BUILTIN_ROUNDS = QUESTION_ROUNDS.slice();

// =============================================================================
//  HIGH SCORE FUNCTIONS
// =============================================================================
function load_scores() {
    var scores = [];
    var f = new File(CFG.score_file);
    if (!f.open("r")) return scores;
    var line;
    while ((line = f.readln(2048)) !== null) {
        var p = line.split("|");
        if (p.length >= 3) {
            scores.push({ name: p[0], score: parseInt(p[1]), date: p[2] });
        }
    }
    f.close();
    return scores;
}

function save_scores(scores) {
    scores.sort(function(a, b) { return b.score - a.score; });
    if (scores.length > CFG.max_scores) scores.length = CFG.max_scores;
    var f = new File(CFG.score_file);
    if (!f.open("w")) return false;
    for (var i = 0; i < scores.length; i++) {
        f.writeln(scores[i].name + "|" + scores[i].score + "|" + scores[i].date);
    }
    f.close();
    return true;
}

function add_score(name, score) {
    var scores = load_scores();
    var d = new Date();
    var ds = (d.getMonth()+1) + "/" + d.getDate() + "/" + d.getFullYear();
    scores.push({ name: name, score: score, date: ds });
    save_scores(scores);
}

function qualifies_for_high_score(score) {
    if (score <= 0) return false;
    var scores = load_scores();
    if (scores.length < CFG.max_scores) return true;
    scores.sort(function(a,b) { return b.score - a.score; });
    return score > scores[scores.length - 1].score;
}

// =============================================================================
//  DRAW: TITLE BANNER (row 1)
// =============================================================================
function draw_title_banner() {
    gotoxy(BOARD_X, ROW_TITLE);
    // Full-width title using block chars
    var bg  = CFG.c_board_bg;
    var fg  = CFG.c_title_fg;
    var w   = BOARD_W;

    // Top accent row
    console.write(clr(fg, bg, true));
    console.write(B.LEFT + rep(B.UPPER, w - 2) + B.RIGHT);
    console.write("\r\n");

    // Main title row: spaced letters
    var title_text = pad_center("J  E  O  P  A  R  D  Y  !", w);
    console.write(clr(fg, bg, true) + title_text + "\r\n");

    // Subtitle row
    var sub_text = pad_center(CFG.subtitle + "  [v" + VERSION + "]", w);
    console.write(clr(7, bg, false) + sub_text + "\r\n");

    // Bottom accent row
    console.write(clr(fg, bg, true));
    console.write(B.LEFT + rep(B.LOWER, w - 2) + B.RIGHT);
}

// =============================================================================
//  DRAW: BOARD GRID
// =============================================================================
// Draws a complete horizontal border line
function draw_h_line(y, lc, mc, rc) {
    gotoxy(BOARD_X, y);
    console.write(clr(CFG.c_border_fg, CFG.c_board_bg, false));
    var s = lc;
    for (var c = 0; c < N_COLS; c++) {
        s += rep(B.H, COL_W);
        s += (c < N_COLS - 1) ? mc : rc;
    }
    console.write(s);
}

function draw_top_line(y)    { draw_h_line(y, B.TL, B.TM, B.TR); }
function draw_mid_line(y)    { draw_h_line(y, B.ML, B.XX, B.MR); }
function draw_bottom_line(y) { draw_h_line(y, B.BL, B.BM, B.BR); }

// Draws a content row with per-cell text/color
// cells: array of { text, fg, bg, bright }
function draw_content_row(y, cells) {
    gotoxy(BOARD_X, y);
    var bfg = CFG.c_border_fg;
    var bbg = CFG.c_board_bg;
    var s   = clr(bfg, bbg, false) + B.V;
    for (var c = 0; c < N_COLS; c++) {
        var cell = cells[c];
        s += clr(cell.fg, cell.bg, cell.bright || false);
        s += pad_center(cell.text || "", COL_W);
        s += clr(bfg, bbg, false) + B.V;
    }
    console.write(s);
}

function draw_board(game) {
    var cats = game.round_data.categories;
    var bg   = CFG.c_board_bg;

    // Title banner (4 rows)
    draw_title_banner();

    var y = ROW_TOP + 3;  // skip 4 title rows (banner is 4 lines)

    // Top border of grid
    draw_top_line(y); y++;

    // Category name rows (2 lines each)
    var cells1 = [], cells2 = [];
    for (var c = 0; c < N_COLS; c++) {
        var name = cats[c].name;
        var l1, l2;
        if (name.length <= COL_W) {
            l1 = name; l2 = "";
        } else {
            // Split at space near midpoint
            var mid = Math.floor(name.length / 2);
            var sp  = name.indexOf(" ", mid);
            if (sp < 0) sp = name.lastIndexOf(" ", mid);
            if (sp < 0) sp = COL_W;
            l1 = name.substring(0, sp).trim();
            l2 = name.substring(sp).trim();
        }
        cells1.push({ text: l1, fg: CFG.c_cat_fg, bg: bg, bright: true });
        cells2.push({ text: l2, fg: CFG.c_cat_fg, bg: bg, bright: false });
    }
    draw_content_row(y, cells1); y++;
    draw_content_row(y, cells2); y++;
    draw_mid_line(y); y++;

    // Value rows
    for (var v = 0; v < CFG.values.length; v++) {
        var val_row = [];
        for (var cc = 0; cc < N_COLS; cc++) {
            var used  = game.used[cc][v];
            var issel = (game.sel_col === cc && game.sel_row === v);
            var isdd  = (game.daily_doubles[cc] === v);

            var text, rfg, rbg, rbright;
            if (used) {
                text    = "";
                rfg     = CFG.c_used_fg;
                rbg     = bg;
                rbright = false;
            } else if (issel) {
                text    = "$" + CFG.values[v];
                rfg     = CFG.c_sel_fg;
                rbg     = CFG.c_sel_bg;
                rbright = true;
            } else {
                text    = "$" + CFG.values[v];
                rfg     = CFG.c_val_fg;
                rbg     = bg;
                rbright = true;
            }
            val_row.push({ text: text, fg: rfg, bg: rbg, bright: rbright });
        }
        draw_content_row(y, val_row); y++;

        if (v < CFG.values.length - 1) {
            draw_mid_line(y); y++;
        }
    }

    // Bottom border
    draw_bottom_line(y); y++;

    // Score bar
    draw_score_bar(y, game);   y++;

    // Navigation help
    gotoxy(BOARD_X, y);
    console.write(clr(7, 0, false));
    console.write(pad_right(
        " \xAF Arrows/WASD: Navigate  " +
        "\xAF ENTER: Select  " +
        "\xAF Q: Quit  " +
        "\xAF H: High Scores",
        BOARD_W));
}

function draw_score_bar(y, game) {
    var bg  = CFG.c_board_bg;
    var n   = game.players.length;
    var cur = game.cur_player;

    // Each player slot width
    var slot_w = Math.floor((BOARD_W - 2) / n);

    gotoxy(BOARD_X, y);
    console.write(clr(7, bg, false));

    for (var pi = 0; pi < n; pi++) {
        var p    = game.players[pi];
        var is_cur = (pi === cur);
        var scfg = p.score < 0 ? clr(1, bg, true) : clr(CFG.c_score_fg, bg, true);
        var namefg = is_cur ? clr(0, CFG.c_score_fg, true) : clr(7, bg, false);
        var label  = (is_cur ? "\x10 " : "  ") +
                     p.name.substring(0, 10) + ": " + format_money(p.score) +
                     (is_cur ? " \x11" : "  ");
        label = pad_center(label, slot_w);
        if (is_cur) {
            console.write(namefg + label);
        } else {
            console.write(scfg + label);
        }
    }
    // Fill any remainder
    console.write(rep(" ", BOARD_W - slot_w * n));
}

// =============================================================================
//  DRAW: CLUE POPUP
// =============================================================================
function draw_clue_box(col, row, game) {
    var clue    = game.round_data.categories[col].clues[row];
    var cat     = game.round_data.categories[col].name;
    var value   = CFG.values[row];
    var is_dd   = (game.daily_doubles[col] === row);
    var bg      = CFG.c_board_bg;

    var bw = 62;
    var bh = (CFG.hint_mode > 0) ? 16 : 14;   // extra 2 rows for hint
    var bx = Math.floor((80 - bw) / 2) + 1;
    var by = Math.floor((24 - bh) / 2) - 1;

    // Fill background
    for (var i = 0; i < bh; i++) {
        gotoxy(bx, by + i);
        console.write(clr(7, bg, false) + rep(" ", bw));
    }

    // Draw border
    var bfg = is_dd ? CFG.c_dd_fg : CFG.c_val_fg;

    function box_line(y2, lc, mc, rc, fill) {
        gotoxy(bx, y2);
        console.write(clr(bfg, bg, true) + lc + rep(fill, bw - 2) + rc);
    }

    box_line(by,       B.TL, B.TM, B.TR, B.H);
    box_line(by+2,     B.ML, B.XX, B.MR, B.H);
    box_line(by+bh-2,  B.ML, B.XX, B.MR, B.H);
    box_line(by+bh-1,  B.BL, B.BM, B.BR, B.H);

    // Fill side borders
    for (var si = 1; si < bh - 1; si++) {
        if (si === 2 || si === bh - 2) continue;
        gotoxy(bx, by + si);
        console.write(clr(bfg, bg, true) + B.V);
        gotoxy(bx + bw - 1, by + si);
        console.write(B.V);
    }

    // Header row
    var hdr;
    if (is_dd) {
        hdr = clr(CFG.c_dd_fg, bg, true) + pad_center("* * *  D A I L Y   D O U B L E  * * *", bw - 2);
    } else {
        hdr = clr(bfg, bg, true) + pad_center(cat + "  \xF9  $" + value, bw - 2);
    }
    gotoxy(bx + 1, by + 1);
    console.write(hdr);

    // Clue text (word-wrapped, up to 5 lines)
    var lines = word_wrap(clue.q, bw - 4);
    var cy    = by + 3;
    for (var li = 0; li < Math.min(lines.length, 5); li++) {
        gotoxy(bx + 1, cy + li);
        console.write(clr(CFG.c_clue_fg, bg, true) + pad_center(lines[li], bw - 2));
    }
    // Blank fill
    for (var fi = Math.min(lines.length, 5); fi < 5; fi++) {
        gotoxy(bx + 1, cy + fi);
        console.write(rep(" ", bw - 2));
    }

    // Hint rows (shown only when hint_mode > 0)
    if (CFG.hint_mode > 0) {
        var hint_str = make_hint(clue.a, CFG.hint_mode);
        var hint_label = (CFG.hint_mode === 1) ? "Answer:" : "Hint:";
        var hint_lines = word_wrap(hint_str, bw - 4);
        var hy = cy + 5;
        gotoxy(bx + 1, hy);
        console.write(clr(6, bg, true) + pad_center(
            hint_label + "  " + (hint_lines[0] || ""), bw - 2));
        gotoxy(bx + 1, hy + 1);
        console.write(clr(6, bg, false) + pad_center(
            hint_lines[1] || "", bw - 2));
    }

    return { bx: bx, by: by, bw: bw, bh: bh, clue: clue, value: value, is_dd: is_dd };
}

// =============================================================================
//  CLUE INTERACTION: DAILY DOUBLE WAGER
// =============================================================================
function get_wager(game, bi) {
    var bx = bi.bx, by = bi.by, bw = bi.bw;
    var bg = CFG.c_board_bg;

    var max_wager = Math.max(game.score, Math.max.apply(null, CFG.values));
    var msg = "WAGER: $1 - " + format_money(max_wager);

    gotoxy(bx + 1, by + bi.bh - 3);
    console.write(clr(CFG.c_dd_fg, bg, true) + pad_center(msg, bw - 2));

    gotoxy(bx + 1, by + bi.bh - 4);
    console.write(clr(7, bg, false) + pad_center("Enter your wager and press ENTER:", bw - 2));

    // Input field
    var input_x = bx + Math.floor(bw / 2) - 5;
    gotoxy(input_x, by + bi.bh - 3);
    console.write(clr(0, 3, true) + rep(" ", 12));
    gotoxy(input_x, by + bi.bh - 3);

    show_cursor();
    var wstr = console.getstr(10, K_NONE);
    hide_cursor();

    var w = parseInt(wstr);
    if (isNaN(w) || w < 1) w = 100;
    if (w > max_wager) w = max_wager;
    return w;
}

// =============================================================================
//  CLUE INTERACTION: TIMED ANSWER INPUT
// =============================================================================
function get_answer(bi, wager) {
    var bx   = bi.bx, by = bi.by, bw = bi.bw, bh = bi.bh;
    var bg   = CFG.c_board_bg;
    var iy   = by + bh - 4;  // input row
    var ty   = by + bh - 3;  // timer row
    var py   = by + bh - 2;  // prompt row

    // Prompt
    gotoxy(bx + 1, py);
    console.write(clr(7, bg, false) + pad_center("Type your answer then press ENTER", bw - 2));

    // Wager display
    if (wager !== undefined) {
        gotoxy(bx + 1, iy - 1);
        console.write(clr(CFG.c_dd_fg, bg, true) +
                      pad_center("Daily Double Wager: " + format_money(wager), bw - 2));
    }

    var answer    = "";
    var max_len   = bw - 8;
    var start_ms  = Date.now();
    var limit_ms  = CFG.time_limit * 1000;
    var done      = false;

    // Draw initial input area
    function redraw_input() {
        gotoxy(bx + 1, iy);
        console.write(clr(0, 3, false) + rep(" ", bw - 2));
        gotoxy(bx + 4, iy);
        console.write(clr(0, 3, false) + ">" + rep(" ", 2) + answer);
    }

    function redraw_timer() {
        var elapsed   = Date.now() - start_ms;
        var remaining = Math.max(0, Math.ceil((limit_ms - elapsed) / 1000));
        var tfg;
        if (remaining > 10) tfg = CFG.c_timer_ok;
        else if (remaining > 4) tfg = CFG.c_timer_warn;
        else tfg = CFG.c_timer_low;

        gotoxy(bx + 1, ty);
        var bar_w  = bw - 18;
        var filled = Math.round((remaining / CFG.time_limit) * bar_w);
        var bar    = rep(B.FULL, filled) + rep(B.LITE, bar_w - filled);
        console.write(clr(tfg, bg, true) +
                      " Time: " + pad_left(String(remaining), 3) + "s [" + bar + "] ");
        return remaining;
    }

    show_cursor();
    redraw_input();
    gotoxy(bx + 7, iy);

    while (!done) {
        var rem = redraw_timer();
        if (rem === 0) break;

        var key = console.inkey(K_NOECHO, 250);
        if (!key || key === "") continue;

        var code = key.charCodeAt(0);

        if (key === "\r" || key === "\n") {
            done = true;
        } else if (code === 8 || code === 127) {
            // Backspace
            if (answer.length > 0) {
                answer = answer.substring(0, answer.length - 1);
                redraw_input();
                gotoxy(bx + 7 + answer.length, iy);
            }
        } else if (key === "\x1b") {
            // Escape = give up / skip
            answer = "";
            done   = true;
        } else if (code >= 32 && code < 127 && answer.length < max_len) {
            answer += key;
            redraw_input();
            gotoxy(bx + 7 + answer.length, iy);
        }
    }

    hide_cursor();
    return answer.trim();
}

// =============================================================================
//  HINT SYSTEM  (hangman-style answer reveal)
// =============================================================================
// mode 0: no hint
// mode 1: underscores only  -- shows word lengths: _ _ _ _ _  _ _ _ _ _ _ _
// mode 2: first letter + underscores -- E _ _ _ _ _  _ _  _ _ _ _ _ _ _ _
function make_hint(answer, mode) {
    if (!mode || mode === 0) return "";
    // Work from the normalized answer so articles etc. are stripped
    var norm = answer.replace(/^(what is |what are |who is |who are |the |a |an )/i, "").trim();
    var words = norm.split(/\s+/);
    var parts = [];
    for (var wi = 0; wi < words.length; wi++) {
        var w = words[wi];
        if (!w) continue;
        var hint = "";
        for (var ci = 0; ci < w.length; ci++) {
            if (mode === 2 && ci === 0) {
                hint += w[0].toUpperCase();
            } else {
                hint += "_";
            }
            if (ci < w.length - 1) hint += " ";
        }
        parts.push(hint);
    }
    return parts.join("   ");
}

// =============================================================================
//  ANSWER CHECKING
// =============================================================================
function normalize(s) {
    s = String(s).toLowerCase().trim();
    s = s.replace(/^(what is |what are |who is |who are |the |a |an )/g, "");
    s = s.replace(/[^a-z0-9 ]/g, " ").replace(/\s+/g, " ").trim();
    return s;
}

function answers_match(given, correct) {
    if (!given || given === "") return false;
    var g = normalize(given);
    var c = normalize(correct);
    if (g === c) return true;
    if (CFG.partial_match) {
        if (g.length >= 3 && c.indexOf(g) >= 0) return true;
        if (c.length >= 3 && g.indexOf(c) >= 0) return true;
    }
    return false;
}

function show_result(bi, correct, given, value, is_correct, answerer, next_player, same_turn) {
    var bx = bi.bx, by = bi.by, bw = bi.bw, bh = bi.bh;
    var bg = CFG.c_board_bg;
    var ry = by + bh - 5;

    var rbg, msg;
    if (is_correct) {
        rbg = CFG.c_correct_bg;
        msg = "\x01 CORRECT! " + answerer + " +" + format_money(value) + " \x01";
    } else if (!given || given === "") {
        rbg = 4;
        msg = "TIME'S UP!  Answer: " + correct;
    } else {
        rbg = CFG.c_wrong_bg;
        msg = "\x18 WRONG! " + answerer + " -" + format_money(value) + "  Ans: " + correct;
    }

    gotoxy(bx + 1, ry);
    console.write(clr(7, rbg, true) + pad_center(msg.substring(0, bw - 4), bw - 2));

    // Next player indicator (multiplayer only)
    var next_line;
    if (!same_turn && next_player) {
        next_line = "\x10 " + next_player + "'s turn next  --  Press any key";
    } else {
        next_line = "Press any key to continue...";
    }
    gotoxy(bx + 1, ry + 1);
    console.write(clr(7, bg, false) + pad_center(next_line, bw - 2));

    console.getkey();
}

// =============================================================================
//  GAME STATE
// =============================================================================
function init_game(round_data, players) {
    var game = {
        round_data:      round_data,
        players:         players,       // array of {name, score}
        cur_player:      0,             // index of player whose turn it is
        sel_col:         0,
        sel_row:         0,
        used:            [],
        daily_doubles:   {},
        clues_answered:  0
    };

    // Init used grid
    for (var c = 0; c < N_COLS; c++) {
        game.used[c] = [];
        for (var r = 0; r < CFG.values.length; r++) {
            game.used[c][r] = false;
        }
    }

    // Place daily doubles
    var placed = 0;
    var attempts = 0;
    while (placed < CFG.daily_doubles && attempts < 1000) {
        attempts++;
        var dc = Math.floor(Math.random() * N_COLS);
        var dr = Math.floor(Math.random() * CFG.values.length);
        if (!(dc in game.daily_doubles)) {
            if (dr === 0 && Math.random() < 0.7) continue;
            game.daily_doubles[dc] = dr;
            placed++;
        }
    }

    return game;
}

function all_used(game) {
    for (var c = 0; c < N_COLS; c++) {
        for (var r = 0; r < CFG.values.length; r++) {
            if (!game.used[c][r]) return false;
        }
    }
    return true;
}

// Move selection, skipping used cells
function move_sel(game, dc, dr) {
    var total = N_COLS * CFG.values.length;
    for (var i = 0; i < total; i++) {
        game.sel_col = (game.sel_col + dc + N_COLS) % N_COLS;
        game.sel_row = (game.sel_row + dr + CFG.values.length) % CFG.values.length;
        if (!game.used[game.sel_col][game.sel_row]) return;
        // Only advance in the intended direction
        if (dc !== 0) dc = (dc > 0) ? 1 : -1;
        if (dr !== 0) dr = (dr > 0) ? 1 : -1;
    }
}

// =============================================================================
//  PLAY ONE ROUND
// =============================================================================
function play_round(round_data, players) {
    calc_rows();
    var game = init_game(round_data, players);

    // Skip to first unused cell
    while (game.used[game.sel_col][game.sel_row] && !all_used(game)) {
        game.sel_col = (game.sel_col + 1) % N_COLS;
    }

    hide_cursor();
    clrscr();
    draw_board(game);

    while (!all_used(game)) {
        var key = console.getkey();

        if (key === "Q" || key === "q") break;
        if (key === "H" || key === "h") {
            show_high_scores();
            clrscr();
            draw_board(game);
            continue;
        }

        var moved = false;
        if (key === "\x1b") {
            var k2 = console.inkey(K_NOECHO, 100);
            if (k2 === "[") {
                var k3 = console.inkey(K_NOECHO, 100);
                if      (k3 === "A") { move_sel(game,  0, -1); moved = true; }
                else if (k3 === "B") { move_sel(game,  0,  1); moved = true; }
                else if (k3 === "C") { move_sel(game,  1,  0); moved = true; }
                else if (k3 === "D") { move_sel(game, -1,  0); moved = true; }
            }
        } else if (key === "w" || key === "W" || key === "8") { move_sel(game,  0, -1); moved = true; }
        else if   (key === "s" || key === "S" || key === "2") { move_sel(game,  0,  1); moved = true; }
        else if   (key === "d" || key === "D" || key === "6") { move_sel(game,  1,  0); moved = true; }
        else if   (key === "a" || key === "A" || key === "4") { move_sel(game, -1,  0); moved = true; }

        if (moved) {
            draw_board(game);
            continue;
        }

        if ((key === "\r" || key === "\n" || key === " ") &&
             !game.used[game.sel_col][game.sel_row]) {

            var col = game.sel_col;
            var row = game.sel_row;
            var bi  = draw_clue_box(col, row, game);

            var wager = bi.value;
            if (bi.is_dd) {
                wager = get_wager(game, bi);
                draw_clue_box(col, row, game);
            }

            var given      = get_answer(bi, bi.is_dd ? wager : undefined);
            var is_correct = answers_match(given, bi.clue.a);
            var cur        = game.cur_player;

            if (is_correct) {
                game.players[cur].score += wager;
                // Correct: same player keeps control
            } else {
                if (CFG.allow_negative) {
                    game.players[cur].score -= wager;
                }
                // Wrong or timeout: pass to next player
                game.cur_player = (cur + 1) % game.players.length;
            }

            show_result(bi, bi.clue.a, given, wager, is_correct,
                        game.players[cur].name, game.players[game.cur_player].name,
                        is_correct || game.players.length === 1);

            game.used[col][row] = true;
            game.clues_answered++;

            // Advance selection
            if (row < CFG.values.length - 1) {
                game.sel_row = row + 1;
            }
            while (!all_used(game) && game.used[game.sel_col][game.sel_row]) {
                game.sel_col = (game.sel_col + 1) % N_COLS;
                if (game.sel_col === 0) {
                    game.sel_row = (game.sel_row + 1) % CFG.values.length;
                }
            }

            clrscr();
            draw_board(game);
        }
    }

    return game.players;
}

// =============================================================================
//  HIGH SCORES DISPLAY
// =============================================================================
function show_high_scores() {
    clrscr();
    var bg  = CFG.c_board_bg;
    var fg  = CFG.c_val_fg;
    var bw  = 60;
    var bx  = Math.floor((80 - bw) / 2) + 1;

    var scores = load_scores();
    scores.sort(function(a, b) { return b.score - a.score; });
    var show_count = Math.min(scores.length, 15);

    // bh: top border + title + sep + col header + sep + rows + sep + footer + bottom border
    var bh = 2 + 3 + show_count + 3;
    var by = Math.floor((24 - bh) / 2);
    if (by < 1) by = 1;
    var inner = bw - 2;  // 58 visible chars between borders

    // Fill whole screen
    for (var fy = 1; fy <= 24; fy++) {
        gotoxy(1, fy);
        console.write(clr(7, bg, false) + rep(" ", 80) + RESET);
    }

    var y = by;

    // Top border
    full_sep(y++, bx, bw, bg, fg, B.TL, B.TM, B.TR);

    // Title
    full_line(y++, bx, bw, bg, fg,
        clr(0, fg, true) + pad_center(" J E O P A R D Y !   H I G H   S C O R E S ", inner));

    // Title/header separator
    full_sep(y++, bx, bw, bg, fg, B.ML, B.XX, B.MR);

    // Column headers  (5+25+14+12+2 = 58 = inner)
    full_line(y++, bx, bw, bg, fg,
        clr(7, bg, true) +
        pad_right("  #",   5) +
        pad_right("  NAME", 25) +
        pad_left("SCORE",  14) +
        pad_left("DATE",   12) +
        "  ");

    // Header/rows separator
    full_sep(y++, bx, bw, bg, fg, B.ML, B.XX, B.MR);

    // Score rows
    if (scores.length === 0) {
        full_line(y++, bx, bw, bg, fg,
            clr(7, bg, false) + pad_center("No scores recorded yet!", inner));
    } else {
        for (var i = 0; i < show_count; i++) {
            var s   = scores[i];
            var sfg = s.score >= 0 ? clr(fg, bg, true) : clr(1, bg, true);
            var rfg = (i === 0)    ? clr(3, bg, true)  : clr(7, bg, false);
            // 5+25+14+12+2 = 58
            full_line(y++, bx, bw, bg, fg,
                rfg + pad_right("  " + (i + 1) + ".", 5) +
                clr(7, bg, false) + pad_right("  " + s.name.substring(0, 21), 25) +
                sfg  + pad_left(format_money(s.score), 14) +
                clr(7, bg, false) + pad_left(s.date, 12) +
                "  ");
        }
    }

    // Rows/footer separator
    full_sep(y++, bx, bw, bg, fg, B.ML, B.XX, B.MR);

    // Footer prompt
    full_line(y++, bx, bw, bg, fg,
        clr(7, bg, false) + pad_center("Press any key to continue...", inner));

    // Bottom border
    full_sep(y, bx, bw, bg, fg, B.BL, B.BM, B.BR);

    console.getkey();
}

// =============================================================================
//  FINAL SCORE SCREEN  (multiplayer podium)
// =============================================================================
function show_final_score(players) {
    clrscr();
    var bg    = CFG.c_board_bg;
    var fg    = CFG.c_val_fg;
    var bw    = 60;
    var bx    = Math.floor((80 - bw) / 2) + 1;
    var inner = bw - 2;  // 58 visible chars between borders

    // Sort descending
    var ranked = [];
    for (var pi = 0; pi < players.length; pi++) {
        ranked.push({ name: players[pi].name, score: players[pi].score });
    }
    ranked.sort(function(a, b) { return b.score - a.score; });

    // bh: top + title + sep + player rows + sep + msg + prompt + bottom
    var bh = ranked.length + 7;
    var by = Math.floor((24 - bh) / 2);
    if (by < 1) by = 1;

    // Fill whole screen
    for (var fy = 1; fy <= 24; fy++) {
        gotoxy(1, fy);
        console.write(clr(7, bg, false) + rep(" ", 80) + RESET);
    }

    var y = by;
    var place_labels = ["1ST","2ND","3RD","4TH","5TH","6TH","7TH","8TH"];
    var place_chars  = ["\x0F","\xF8","\xF9","\xFA","\xFA","\xFA","\xFA","\xFA"];

    full_sep(y++, bx, bw, bg, fg, B.TL, B.TM, B.TR);

    // Title (inner=58)
    full_line(y++, bx, bw, bg, fg,
        clr(0, fg, true) + pad_center(" F I N A L   S C O R E S ", inner));

    full_sep(y++, bx, bw, bg, fg, B.ML, B.XX, B.MR);

    // Player rows: 2+1+1+5+23+13+13=58
    for (var ri = 0; ri < ranked.length; ri++) {
        var p      = ranked[ri];
        var is_win = (ri === 0);
        var pfg    = is_win ? clr(3, bg, true)  : clr(7, bg, false);
        var scfg   = p.score < 0 ? clr(1, bg, true) :
                     (is_win ? clr(3, bg, true) : clr(fg, bg, false));
        var medal  = place_chars[ri]  || " ";
        var place  = place_labels[ri] || ((ri + 1) + "TH");
        full_line(y++, bx, bw, bg, fg,
            clr(3, bg, true)  + "  " + medal + " " + pad_right(place, 5) +
            pfg               + pad_right(p.name.substring(0, 23), 23) +
            scfg              + pad_left(format_money(p.score), 13) +
            clr(7, bg, false) + rep(" ", inner - 44));
    }

    full_sep(y++, bx, bw, bg, fg, B.ML, B.XX, B.MR);

    // Winner message
    var winner = ranked[0];
    var hs_msg;
    if (qualifies_for_high_score(winner.score)) {
        hs_msg = clr(2, bg, true) +
            pad_center("\x0F  " + winner.name.substring(0, 18) + ": NEW HIGH SCORE!  \x0F", inner);
        add_score(winner.name, winner.score);
    } else if (winner.score > 0) {
        hs_msg = clr(3, bg, true) +
            pad_center(winner.name.substring(0, 20) + " wins!  Congratulations!", inner);
    } else {
        hs_msg = clr(7, bg, false) + pad_center("Better luck next time!", inner);
    }
    full_line(y++, bx, bw, bg, fg, hs_msg);

    // Save qualifying runner-up scores
    for (var qi = 1; qi < ranked.length; qi++) {
        if (qualifies_for_high_score(ranked[qi].score)) {
            add_score(ranked[qi].name, ranked[qi].score);
        }
    }

    // Prompt
    full_line(y++, bx, bw, bg, fg,
        clr(7, bg, false) + pad_center("Press any key to return to the main menu...", inner));

    // Bottom border
    full_sep(y, bx, bw, bg, fg, B.BL, B.BM, B.BR);

    console.getkey();
}

// =============================================================================
//  PLAYER SETUP SCREEN
// =============================================================================
function setup_players(default_name) {
    var max_p = Math.max(1, Math.min(CFG.max_players, 8));
    var bg    = CFG.c_board_bg;
    var fg    = CFG.c_val_fg;
    var bw    = 52;
    var bx    = Math.floor((80 - bw) / 2) + 1;
    var by    = 2;
    var inner = bw - 2;  // usable width inside borders

    // Helper: draw one full content row inside the box
    function box_row(y, text_seq) {
        gotoxy(bx, y);
        console.write(clr(fg, bg, true) + B.V + text_seq +
                      clr(fg, bg, true) + B.V);
    }
    function blank_row(y) {
        box_row(y, clr(7, bg, false) + rep(" ", inner));
    }

    // -- PHASE 1: ask number of players --------------------------------------
    function draw_phase1() {
        clrscr();
        gotoxy(bx, by);
        console.write(clr(fg, bg, true) + B.TL + rep(B.H, inner) + B.TR);
        gotoxy(bx, by + 1);
        console.write(B.V + clr(0, fg, true) +
                      pad_center(" PLAYER SETUP ", inner) +
                      clr(fg, bg, true) + B.V);
        gotoxy(bx, by + 2);
        console.write(B.ML + rep(B.H, inner) + B.MR);
        blank_row(by + 3);
        box_row(by + 4, clr(7, bg, false) +
                pad_center("How many players? (1-" + max_p + ")", inner));
        blank_row(by + 5);
        box_row(by + 6, clr(3, bg, true) +
                pad_center("Enter number and press ENTER:", inner));
        // Input slot - highlighted field centered
        var slot_x = bx + Math.floor(inner / 2) - 1;
        gotoxy(bx, by + 7);
        console.write(clr(fg, bg, true) + B.V +
                      clr(7, bg, false) + rep(" ", Math.floor(inner / 2) - 2) +
                      clr(0, 7, true)   + "    " +
                      clr(7, bg, false) + rep(" ", inner - Math.floor(inner / 2) - 2) +
                      clr(fg, bg, true) + B.V);
        blank_row(by + 8);
        gotoxy(bx, by + 9);
        console.write(clr(fg, bg, true) + B.BL + rep(B.H, inner) + B.BR);
        return slot_x;
    }

    var slot_x = draw_phase1();
    gotoxy(slot_x, by + 7);
    show_cursor();
    var np_str = console.getstr(2, K_NONE);
    hide_cursor();
    var np = parseInt(np_str);
    if (isNaN(np) || np < 1) np = 1;
    if (np > max_p) np = max_p;

    // -- PHASE 2: enter player names -----------------------------------------
    // Box height: title(3) + blank + "enter names"(1) + blank + np rows + blank + ready(1) + bottom = 8 + np
    var total_rows = 9 + np;

    function draw_phase2(players_so_far) {
        clrscr();
        gotoxy(bx, by);
        console.write(clr(fg, bg, true) + B.TL + rep(B.H, inner) + B.TR);
        gotoxy(bx, by + 1);
        console.write(B.V + clr(0, fg, true) +
                      pad_center(" PLAYER SETUP  -  " + np + " Player" + (np > 1 ? "s" : ""), inner) +
                      clr(fg, bg, true) + B.V);
        gotoxy(bx, by + 2);
        console.write(B.ML + rep(B.H, inner) + B.MR);
        blank_row(by + 3);
        box_row(by + 4, clr(7, bg, false) +
                pad_center("Enter a name and press ENTER for each player:", inner));
        blank_row(by + 5);

        // Player name rows
        for (var pi = 0; pi < np; pi++) {
            var row_y   = by + 6 + pi;
            var entered = (pi < players_so_far.length) ? players_so_far[pi].name : "";
            var is_cur  = (pi === players_so_far.length);
            var prefix  = "  Player " + (pi + 1) + ":  ";
            // prefix is 13 chars, input field is inner - 15 chars
            var field_w = inner - prefix.length - 2;
            var field_content;
            if (entered) {
                field_content = clr(2, bg, true) + pad_right(entered, field_w);
            } else if (is_cur) {
                field_content = clr(0, 7, false) + pad_right("", field_w);
            } else {
                field_content = clr(7, bg, false) + pad_right("", field_w);
            }
            gotoxy(bx, row_y);
            console.write(clr(fg, bg, true) + B.V +
                          clr(3, bg, true)  + prefix +
                          field_content +
                          clr(7, bg, false) + "  " +
                          clr(fg, bg, true) + B.V);
        }

        // Blank + separator + ready row + bottom
        blank_row(by + 6 + np);
        gotoxy(bx, by + 7 + np);
        console.write(clr(fg, bg, true) + B.ML + rep(B.H, inner) + B.MR);
        gotoxy(bx, by + 8 + np);
        var ready_txt;
        if (players_so_far.length === np) {
            var names = [];
            for (var ri = 0; ri < players_so_far.length; ri++) {
                names.push(players_so_far[ri].name);
            }
            ready_txt = clr(2, bg, true) + pad_center("Ready: " + names.join(", "), inner);
        } else {
            ready_txt = clr(7, bg, false) + pad_center("", inner);
        }
        console.write(clr(fg, bg, true) + B.V + ready_txt + clr(fg, bg, true) + B.V);
        gotoxy(bx, by + 9 + np);
        console.write(B.BL + rep(B.H, inner) + B.BR);
    }

    var players = [];
    for (var pi = 0; pi < np; pi++) {
        draw_phase2(players);

        // Position cursor at the current player's input field
        var prefix_len = ("  Player " + (pi + 1) + ":  ").length;
        var input_x    = bx + 1 + prefix_len;
        var input_y    = by + 6 + pi;
        var field_w2   = inner - prefix_len - 2;
        var def_name   = (pi === 0 && default_name) ? default_name : ("Player " + (pi + 1));

        gotoxy(input_x, input_y);
        show_cursor();
        var pname = console.getstr(Math.min(field_w2, 20), K_NONE);
        hide_cursor();
        if (!pname || pname.trim() === "") pname = def_name;
        players.push({ name: pname.trim(), score: 0 });
    }

    // Final draw with all names filled in
    draw_phase2(players);
    gotoxy(bx, by + 9 + np + 2);
    console.write(clr(7, 0, false) + pad_center("Press any key to start!", bw));
    console.getkey();

    return players;
}

// =============================================================================
//  INSTRUCTIONS SCREEN
// =============================================================================
function show_instructions() {
    clrscr();
    var bg  = CFG.c_board_bg;
    var fg  = CFG.c_val_fg;
    var bw  = 68;
    var bx  = Math.floor((80 - bw) / 2) + 1;
    var inner = bw - 2;

    var hint_line;
    if (CFG.hint_mode === 0) {
        hint_line = "  Answer hints are DISABLED.";
    } else if (CFG.hint_mode === 1) {
        hint_line = "  HINT MODE: Underscores show you the answer's word lengths.";
    } else {
        hint_line = "  HINT MODE: First letters + underscores reveal the answer shape.";
    }

    var lines = [
        "",
        "  Navigate the board with ARROW KEYS or W/A/S/D.",
        "  Press ENTER or SPACE to select a highlighted clue.",
        "  Type your answer and press ENTER to submit.",
        "  Press ESC during a clue to skip / give up.",
        "",
        "  MULTIPLAYER: Up to " + CFG.max_players + " players can play hot-seat.",
        "  Enter names at the start of each game. The active",
        "  player is highlighted in the score bar at the bottom.",
        "  Correct answer = keep control. Wrong = next player's turn.",
        "  You have " + CFG.time_limit + " seconds to answer each clue.",
        "  Correct answers ADD the dollar value to your score.",
        (CFG.allow_negative ?
        "  Wrong answers SUBTRACT the dollar value from your score." :
        "  Wrong answers do NOT deduct from your score."),
        "",
        "  Watch out for DAILY DOUBLES! You can wager up to your score.",
        "",
        "  Answers are NOT case-sensitive. Articles like 'The', 'A', 'An'",
        "  at the start of answers are stripped automatically.",
        (CFG.partial_match ?
        "  Partial answers (substrings) are accepted." :
        "  You must give the complete answer."),
        "",
        hint_line,
        "",
        "  Press Q at any time to quit.  Press H for high scores.",
        ""
    ];

    var bh = lines.length + 4;  // top + title + sep + lines + bottom
    var by = Math.floor((24 - bh) / 2);
    if (by < 1) by = 1;

    // Fill screen
    for (var fy = 1; fy <= 24; fy++) {
        gotoxy(1, fy);
        console.write(clr(7, bg, false) + rep(" ", 80) + RESET);
    }

    var y = by;
    full_sep(y++, bx, bw, bg, fg, B.TL, B.TM, B.TR);
    full_line(y++, bx, bw, bg, fg,
        clr(0, fg, true) + pad_center(" H O W   T O   P L A Y ", inner));
    full_sep(y++, bx, bw, bg, fg, B.ML, B.XX, B.MR);

    for (var i = 0; i < lines.length; i++) {
        full_line(y++, bx, bw, bg, fg,
            clr(7, bg, false) + pad_right(lines[i], inner));
    }

    full_sep(y++, bx, bw, bg, fg, B.ML, B.XX, B.MR);
    full_line(y++, bx, bw, bg, fg,
        clr(7, bg, false) + pad_center("Press any key to continue...", inner));
    full_sep(y, bx, bw, bg, fg, B.BL, B.BM, B.BR);

    console.getkey();
}

// =============================================================================
//  MAIN MENU
// =============================================================================
function show_main_menu() {
    clrscr();
    var bg  = CFG.c_board_bg;
    var fg  = CFG.c_val_fg;
    var w   = BOARD_W;

    // Jeopardy logo block
    var lines_logo = [
        clr(fg, bg, true)  + B.LEFT + rep(B.UPPER, w - 2) + B.RIGHT,
        clr(fg, bg, true)  + B.V    + clr(0, fg, true) +
            pad_center(" J  E  O  P  A  R  D  Y ! ", w - 2) +
            clr(fg, bg, true) + B.V,
        clr(fg, bg, true)  + B.V    + clr(7, bg, false) +
            pad_center(CFG.subtitle, w - 2) +
            clr(fg, bg, true) + B.V,
        clr(fg, bg, true)  + B.LEFT + rep(B.LOWER, w - 2) + B.RIGHT
    ];
    for (var li = 0; li < lines_logo.length; li++) {
        gotoxy(BOARD_X, 2 + li);
        console.write(lines_logo[li]);
    }

    // Menu box
    var bw  = 38;
    var bx  = Math.floor((80 - bw) / 2) + 1;
    var by  = 9;

    // Check if current user is sysop
    var is_sysop = false;
    try {
        if (typeof user !== "undefined" && user &&
            typeof user.security !== "undefined" &&
            user.security.level >= CFG.sysop_level) {
            is_sysop = true;
        }
    } catch(e) {}

    var items = [
        { key: "1", label: QUESTION_ROUNDS.length === 0 ?
            "Play Jeopardy!  [!] NO QUESTIONS LOADED" : "Play Jeopardy!" },
        { key: "2", label: "View High Scores" },
        { key: "3", label: "Instructions" }
    ];
    if (is_sysop) {
        items.push({ key: "S", label: "Sysop Menu \xAE" });
    }
    items.push({ key: "Q", label: "Quit / Return to BBS" });

    gotoxy(bx, by);
    console.write(clr(fg, bg, true) + B.TL + rep(B.H, bw - 2) + B.TR);

    for (var mi = 0; mi < items.length; mi++) {
        gotoxy(bx, by + 1 + mi);
        console.write(clr(fg, bg, true) + B.V +
                      clr(fg, bg, true) + "  [" + items[mi].key + "]  " +
                      clr(7, bg, false) + pad_right(items[mi].label, bw - 9) +
                      clr(fg, bg, true) + B.V);
    }

    gotoxy(bx, by + 1 + items.length);
    console.write(B.BL + rep(B.H, bw - 2) + B.BR);

    // Version/credit footer
    gotoxy(BOARD_X, 22);
    console.write(clr(7, 0, false) +
                  pad_center("Jeopardy! BBS Edition v" + VERSION +
                             "  \xF9  " + QUESTION_ROUNDS.length + " Rounds Available", w));
    gotoxy(BOARD_X, 23);
    var hint_label = ["No Hints","Underscores","Letters+Underscores"];
    var footer2 = "Hint Mode: " + hint_label[CFG.hint_mode || 0];
    if (is_sysop) footer2 += "  \xAE  Sysop: Press S for admin menu";
    console.write(clr(is_sysop ? 5 : 7, 0, is_sysop) + pad_center(footer2, w));

    // Input prompt
    gotoxy(bx + 4, by + 1 + items.length + 2);
    console.write(clr(7, 0, false) + "Your choice: ");
    show_cursor();
    var key = console.getkey();
    hide_cursor();
    return key;
}

// =============================================================================
//  EXTERNAL QUESTIONS FILE LOADER
// =============================================================================
function try_load_external_questions() {
    var path = CFG.questions_file;

    // If path has no directory separator, prepend GAME_DIR
    if (path.indexOf("/") < 0 && path.indexOf("\\") < 0) {
        path = GAME_DIR + path;
    }

    var f = new File(path);
    if (!f.open("r")) {
        // Try the bare path as a last resort
        f = new File(CFG.questions_file);
        if (!f.open("r")) return;
    }

    // Read entire file - use large buffer per line to handle big JSON lines
    var raw = "";
    var line;
    while ((line = f.readln(65535)) !== null) raw += line + "\n";
    f.close();

    raw = raw.trim();
    if (!raw) return;

    try {
        var ext = JSON.parse(raw);
        if (Array.isArray(ext) && ext.length > 0) {
            EXT_ROUNDS = ext;
            for (var i = 0; i < ext.length; i++) {
                QUESTION_ROUNDS.push(ext[i]);
            }
        }
    } catch (e) {
        // Show a visible error so the sysop knows the file failed to parse
        clrscr();
        console.writeln("");
        console.writeln("WARNING: Could not parse questions.json");
        console.writeln("File: " + path);
        console.writeln("Error: " + e);
        console.writeln("");
        console.writeln("Press any key to continue without external questions...");
        console.getkey();
    }
}

function save_external_questions() {
    var f = new File(CFG.questions_file);
    if (!f.open("w")) return false;
    f.write(JSON.stringify(EXT_ROUNDS, null, 2));
    f.close();
    return true;
}

function rebuild_question_rounds() {
    // Rebuild QUESTION_ROUNDS from scratch based on current settings
    QUESTION_ROUNDS = [];
    // Re-insert built-in rounds if enabled - they live in BUILTIN_ROUNDS
    if (CFG.use_builtin) {
        for (var bi = 0; bi < BUILTIN_ROUNDS.length; bi++) {
            QUESTION_ROUNDS.push(BUILTIN_ROUNDS[bi]);
        }
    }
    for (var ei = 0; ei < EXT_ROUNDS.length; ei++) {
        QUESTION_ROUNDS.push(EXT_ROUNDS[ei]);
    }
}

// =============================================================================
//  SYSOP MENU HELPERS
// =============================================================================
var COLOR_NAMES = ["Black","Red","Green","Yellow","Blue","Magenta","Cyan","White"];

function sy_box(title, items_count, bw) {
    // Draw a standard sysop-style menu box, returns {bx,by,bw}
    clrscr();
    bw = bw || 60;
    var bx = Math.floor((80 - bw) / 2) + 1;
    var by = 2;
    var bg = 0;
    var fg = 5; // magenta for sysop theme

    gotoxy(bx, by);
    console.write(clr(fg, bg, true) + B.TL + rep(B.H, bw - 2) + B.TR);
    gotoxy(bx, by + 1);
    console.write(B.V + clr(0, fg, true) + pad_center(" JEOPARDY! SYSOP MENU  \xAE  " + title, bw - 2) +
                  clr(fg, bg, true) + B.V);
    gotoxy(bx, by + 2);
    console.write(B.ML + rep(B.H, bw - 2) + B.MR);
    return { bx: bx, by: by, bw: bw, bg: bg, fg: fg };
}

function sy_row(bx, y, bw, key, label, value, bg, fg) {
    gotoxy(bx, y);
    console.write(clr(fg, bg, true) + B.V +
        clr(3, bg, true)  + "  [" + key + "]  " +
        clr(7, bg, false) + pad_right(label, 28) +
        clr(6, bg, true)  + pad_right(value !== undefined ? String(value) : "", bw - 38) + " " +
        clr(fg, bg, true) + B.V);
}

function sy_close(bx, y, bw, fg, bg) {
    gotoxy(bx, y);
    console.write(clr(fg, bg, true) + B.BL + rep(B.H, bw - 2) + B.BR);
    gotoxy(bx, y + 2);
    console.write(clr(7, 0, false) + pad_center("Choice: ", bw));
    gotoxy(bx + Math.floor(bw / 2) + 1, y + 2);
    show_cursor();
    var k = console.getkey();
    hide_cursor();
    return k;
}

function sy_msg(msg, is_err) {
    gotoxy(1, 23);
    var c = is_err ? clr(1, 0, true) : clr(2, 0, true);
    console.write(c + pad_center(msg, 78) + RESET);
    if (is_err) {
        console.inkey(K_NOECHO, 1500);
    }
}

function sy_input(prompt, def_val, max_len) {
    gotoxy(1, 24);
    console.write(clr(7, 0, false) + rep(" ", 78));
    gotoxy(1, 24);
    console.write(clr(3, 0, true) + prompt + clr(0, 7, false));
    show_cursor();
    var v = console.getstr(max_len || 60, K_NONE);
    hide_cursor();
    return (v === "" || v === null) ? def_val : v;
}

// =============================================================================
//  SYSOP MENU - MAIN
// =============================================================================
function sysop_menu() {
    var running = true;
    while (running) {
        var b = sy_box("MAIN", 6, 60);
        var y = b.by + 3;
        sy_row(b.bx, y++, b.bw, "1", "Game Settings",       "", b.bg, b.fg);
        sy_row(b.bx, y++, b.bw, "2", "Color Settings",      "", b.bg, b.fg);
        sy_row(b.bx, y++, b.bw, "3", "Question Editor",     "", b.bg, b.fg);
        sy_row(b.bx, y++, b.bw, "4", "Clear High Scores",   "", b.bg, b.fg);
        sy_row(b.bx, y++, b.bw, "5", "Toggle Built-in Q's",
               CFG.use_builtin ? "[ENABLED]" : "[DISABLED]", b.bg, b.fg);
        sy_row(b.bx, y++, b.bw, "6", "Diagnostics / Path Check", "", b.bg, b.fg);
        sy_row(b.bx, y++, b.bw, "Q", "Back to Main Menu",   "", b.bg, b.fg);

        var k = sy_close(b.bx, y, b.bw, b.fg, b.bg);
        if      (k === "1") { sysop_game_settings(); }
        else if (k === "2") { sysop_color_settings(); }
        else if (k === "3") { sysop_question_editor(); }
        else if (k === "4") { sysop_clear_scores(); }
        else if (k === "5") {
            CFG.use_builtin = !CFG.use_builtin;
            rebuild_question_rounds();
            save_config();
            sy_msg("Built-in questions " + (CFG.use_builtin ? "ENABLED" : "DISABLED") +
                   "  (" + QUESTION_ROUNDS.length + " total rounds now active)");
        }
        else if (k === "6") { sysop_diagnostics(); }
        else if (k === "Q" || k === "q") { running = false; }
    }
}

// =============================================================================
//  SYSOP MENU - GAME SETTINGS
// =============================================================================
function sysop_game_settings() {
    var running = true;
    while (running) {
        var b = sy_box("GAME SETTINGS", 12, 70);
        var y = b.by + 3;
        var bw = b.bw;

        sy_row(b.bx, y++, bw, "1",  "Title",            CFG.title,           b.bg, b.fg);
        sy_row(b.bx, y++, bw, "2",  "Subtitle",         CFG.subtitle,        b.bg, b.fg);
        sy_row(b.bx, y++, bw, "3",  "Dollar Values",    CFG.values.join(","), b.bg, b.fg);
        sy_row(b.bx, y++, bw, "4",  "Time Limit (sec)", CFG.time_limit,       b.bg, b.fg);
        sy_row(b.bx, y++, bw, "5",  "Daily Doubles",    CFG.daily_doubles,    b.bg, b.fg);
        sy_row(b.bx, y++, bw, "6",  "Allow Negative",   CFG.allow_negative ? "YES" : "NO", b.bg, b.fg);
        sy_row(b.bx, y++, bw, "7",  "Partial Match",    CFG.partial_match  ? "YES" : "NO", b.bg, b.fg);
        sy_row(b.bx, y++, bw, "8",  "Max Scores",       CFG.max_scores,       b.bg, b.fg);
        sy_row(b.bx, y++, bw, "P",  "Max Players (1-8)", CFG.max_players,     b.bg, b.fg);
        sy_row(b.bx, y++, bw, "9",  "Hint Mode",
               CFG.hint_mode === 0 ? "0: None" :
               CFG.hint_mode === 1 ? "1: Underscores Only" : "2: Letters+Underscores",
               b.bg, b.fg);
        sy_row(b.bx, y++, bw, "0",  "Sysop Level",      CFG.sysop_level,      b.bg, b.fg);
        sy_row(b.bx, y++, bw, "S",  "Save to jeopardy.ini", "",                b.bg, b.fg);
        sy_row(b.bx, y++, bw, "Q",  "Back",             "",                    b.bg, b.fg);

        var k = sy_close(b.bx, y, bw, b.fg, b.bg);
        var v;
        if (k === "1") {
            v = sy_input("New title: ", CFG.title, 40);
            CFG.title = v;
        } else if (k === "2") {
            v = sy_input("New subtitle: ", CFG.subtitle, 40);
            CFG.subtitle = v;
        } else if (k === "3") {
            v = sy_input("Values (comma-sep, e.g. 200,400,600,800,1000): ",
                         CFG.values.join(","), 50);
            var parts = v.split(",");
            var arr = [];
            for (var pi = 0; pi < parts.length; pi++) {
                var n = parseInt(parts[pi].trim());
                if (!isNaN(n) && n > 0) arr.push(n);
            }
            if (arr.length >= 2) { CFG.values = arr; calc_rows(); }
            else sy_msg("Need at least 2 values", true);
        } else if (k === "4") {
            v = parseInt(sy_input("Time limit in seconds (5-120): ", CFG.time_limit, 4));
            if (!isNaN(v) && v >= 5 && v <= 120) CFG.time_limit = v;
            else sy_msg("Must be 5-120", true);
        } else if (k === "5") {
            v = parseInt(sy_input("Daily doubles per round (0-6): ", CFG.daily_doubles, 2));
            if (!isNaN(v) && v >= 0 && v <= 6) CFG.daily_doubles = v;
            else sy_msg("Must be 0-6", true);
        } else if (k === "6") {
            CFG.allow_negative = !CFG.allow_negative;
        } else if (k === "7") {
            CFG.partial_match = !CFG.partial_match;
        } else if (k === "8") {
            v = parseInt(sy_input("Max score table entries (5-100): ", CFG.max_scores, 4));
            if (!isNaN(v) && v >= 5 && v <= 100) CFG.max_scores = v;
            else sy_msg("Must be 5-100", true);
        } else if (k === "P" || k === "p") {
            v = parseInt(sy_input("Max players per game (1-8): ", CFG.max_players, 2));
            if (!isNaN(v) && v >= 1 && v <= 8) CFG.max_players = v;
            else sy_msg("Must be 1-8", true);
        } else if (k === "9") {
            CFG.hint_mode = (CFG.hint_mode + 1) % 3;
            sy_msg("Hint mode: " +
                   (CFG.hint_mode === 0 ? "NONE" :
                    CFG.hint_mode === 1 ? "Underscores Only" : "Letters + Underscores"));
        } else if (k === "0") {
            v = parseInt(sy_input("Sysop security level (1-255): ", CFG.sysop_level, 4));
            if (!isNaN(v) && v >= 1 && v <= 255) CFG.sysop_level = v;
            else sy_msg("Must be 1-255", true);
        } else if (k === "S" || k === "s") {
            if (save_config()) sy_msg("Saved to jeopardy.ini");
            else sy_msg("ERROR: Could not write jeopardy.ini", true);
        } else if (k === "Q" || k === "q") {
            running = false;
        }
    }
}

// =============================================================================
//  SYSOP MENU - COLOR SETTINGS
// =============================================================================
function sysop_color_settings() {
    var c_defs = [
        ["c_board_bg",  "Board background",      false],
        ["c_border_fg", "Grid borders",           true],
        ["c_cat_fg",    "Category text",          true],
        ["c_val_fg",    "Dollar amounts",         true],
        ["c_used_fg",   "Used cell (hidden)",     false],
        ["c_sel_fg",    "Selected cell fg",       true],
        ["c_sel_bg",    "Selected cell bg",       false],
        ["c_title_fg",  "Title text",             true],
        ["c_clue_fg",   "Clue text",              true],
        ["c_dd_fg",     "Daily Double",           true],
        ["c_correct_bg","Correct answer bar bg",  false],
        ["c_wrong_bg",  "Wrong answer bar bg",    false],
        ["c_score_fg",  "Score display",          true],
        ["c_timer_ok",  "Timer > 10s",            true],
        ["c_timer_warn","Timer 5-10s",            true],
        ["c_timer_low", "Timer < 5s",             true],
    ];

    var page    = 0;
    var per_pg  = 10;
    var running = true;

    while (running) {
        var b  = sy_box("COLORS  (0-7: " + COLOR_NAMES.join("/") + ")", per_pg + 3, 70);
        var y  = b.by + 3;
        var bw = b.bw;
        var start = page * per_pg;
        var end   = Math.min(start + per_pg, c_defs.length);

        for (var ci = start; ci < end; ci++) {
            var cd  = c_defs[ci];
            var key = String(ci - start + 1);
            if (key === "10") key = "0";
            var cv  = CFG[cd[0]];
            var sample = clr(cv, (cd[2] ? 0 : cv), cd[2]) + " " + COLOR_NAMES[cv] + " " + RESET;
            sy_row(b.bx, y++, bw, key, cd[1], "[" + cv + "] " + COLOR_NAMES[cv], b.bg, b.fg);
        }

        if (c_defs.length > per_pg) {
            sy_row(b.bx, y++, bw, "N", "Next page", "(page " + (page+1) + "/" +
                   Math.ceil(c_defs.length/per_pg) + ")", b.bg, b.fg);
        }
        sy_row(b.bx, y++, bw, "S", "Save to jeopardy.ini", "", b.bg, b.fg);
        sy_row(b.bx, y++, bw, "Q", "Back", "", b.bg, b.fg);

        var k = sy_close(b.bx, y, bw, b.fg, b.bg);

        if (k === "N" || k === "n") {
            page = (page + 1) % Math.ceil(c_defs.length / per_pg);
        } else if (k === "S" || k === "s") {
            if (save_config()) sy_msg("Colors saved");
            else sy_msg("ERROR saving", true);
        } else if (k === "Q" || k === "q") {
            running = false;
        } else {
            var idx = (k === "0") ? 9 : parseInt(k) - 1;
            if (!isNaN(idx) && idx >= 0 && idx < (end - start)) {
                var cd2 = c_defs[start + idx];
                var nv = parseInt(sy_input(
                    "New value for " + cd2[1] + " (0-7, current=" + CFG[cd2[0]] + "): ",
                    CFG[cd2[0]], 2));
                if (!isNaN(nv) && nv >= 0 && nv <= 7) CFG[cd2[0]] = nv;
                else sy_msg("Must be 0-7", true);
            }
        }
    }
}

// =============================================================================
//  SYSOP MENU - CLEAR HIGH SCORES
// =============================================================================
function sysop_clear_scores() {
    clrscr();
    gotoxy(1, 10);
    console.write(clr(1, 0, true) + pad_center("WARNING: Delete ALL high scores? (Y/N)", 80));
    gotoxy(1, 12);
    console.write(clr(7, 0, false) + pad_center("This cannot be undone.", 80));
    gotoxy(1, 14);
    console.write(pad_center("Press Y to confirm or any other key to cancel: ", 80));
    show_cursor();
    var k = console.getkey();
    hide_cursor();
    if (k === "Y" || k === "y") {
        var f = new File(CFG.score_file);
        if (f.open("w")) { f.close(); sy_msg("High scores cleared."); }
        else sy_msg("ERROR: Could not clear score file", true);
    } else {
        sy_msg("Cancelled.");
    }
}

// =============================================================================
//  SYSOP MENU - QUESTION EDITOR
// =============================================================================
function sysop_question_editor() {
    var qe_running = true;
    while (qe_running) {
        var b = sy_box("QUESTION EDITOR", 5, 68);
        var y = b.by + 3;
        var bw = b.bw;
        var ext_count = EXT_ROUNDS.length;
        var cat_count = 0;
        for (var ri = 0; ri < EXT_ROUNDS.length; ri++) {
            cat_count += (EXT_ROUNDS[ri].categories || []).length;
        }

        sy_row(b.bx, y++, bw, "1", "Browse/Edit Questions",
               ext_count + " rounds, " + cat_count + " categories", b.bg, b.fg);
        sy_row(b.bx, y++, bw, "2", "Add New Category to Round", "", b.bg, b.fg);
        sy_row(b.bx, y++, bw, "3", "Add New Round",             "", b.bg, b.fg);
        sy_row(b.bx, y++, bw, "4", "Delete a Round",            "", b.bg, b.fg);
        sy_row(b.bx, y++, bw, "Q", "Back",                      "", b.bg, b.fg);

        var k = sy_close(b.bx, y, bw, b.fg, b.bg);
        if      (k === "1") { qe_browse(); }
        else if (k === "2") { qe_add_category(); }
        else if (k === "3") { qe_add_round(); }
        else if (k === "4") { qe_delete_round(); }
        else if (k === "Q" || k === "q") { qe_running = false; }
    }
}

// Pick a round (returns index into EXT_ROUNDS or -1)
function qe_pick_round(prompt) {
    if (EXT_ROUNDS.length === 0) {
        sy_msg("No external rounds loaded. Add one first.", true);
        return -1;
    }
    clrscr();
    var bw = 70;
    var bx = Math.floor((80 - bw) / 2) + 1;
    var bg = 0; var fg = 5;

    gotoxy(bx, 2);
    console.write(clr(fg, bg, true) + B.TL + rep(B.H, bw-2) + B.TR);
    gotoxy(bx, 3);
    console.write(B.V + clr(0, fg, true) + pad_center(prompt, bw-2) + clr(fg,bg,true) + B.V);
    gotoxy(bx, 4);
    console.write(B.ML + rep(B.H, bw-2) + B.MR);

    var per_page = 16;
    var page = 0;
    var n = EXT_ROUNDS.length;

    function draw_rounds() {
        var start = page * per_page;
        var end = Math.min(start + per_page, n);
        var y = 5;
        for (var ri = start; ri < end; ri++) {
            var cats = (EXT_ROUNDS[ri].categories || []);
            var names = [];
            for (var ci2 = 0; ci2 < Math.min(3, cats.length); ci2++) {
                names.push(cats[ci2].name);
            }
            if (cats.length > 3) names.push("...");
            var key = String((ri - start + 1) % 10);
            gotoxy(bx, y++);
            console.write(clr(fg,bg,true) + B.V +
                clr(3,bg,true)  + " [" + pad_left(String(ri+1), 2) + "] " +
                clr(7,bg,false) + pad_right("Round " + (ri+1) + ": " + names.join(", "), bw-9) + " " +
                clr(fg,bg,true) + B.V);
        }
        // fill empty lines
        for (var fi = end - page*per_page; fi < per_page; fi++) {
            gotoxy(bx, 5 + fi);
            console.write(clr(fg,bg,true) + B.V + rep(" ", bw-2) + B.V);
        }
        gotoxy(bx, 5 + per_page);
        console.write(B.ML + rep(B.H, bw-2) + B.MR);
        gotoxy(bx, 6 + per_page);
        console.write(B.V + clr(7,bg,false) +
            pad_center("Type round number + ENTER  |  N=Next Page  |  Q=Cancel", bw-2) +
            clr(fg,bg,true) + B.V);
        gotoxy(bx, 7 + per_page);
        console.write(B.BL + rep(B.H, bw-2) + B.BR);
    }

    while (true) {
        draw_rounds();
        gotoxy(bx + 2, 9 + per_page);
        console.write(clr(3,0,true) + "Round #: " + clr(0,7,false) + "     ");
        gotoxy(bx + 11, 9 + per_page);
        show_cursor();
        var inp = console.getstr(5, K_NONE);
        hide_cursor();
        if (!inp || inp === "Q" || inp === "q") return -1;
        if (inp === "N" || inp === "n") {
            page = (page + 1) % Math.ceil(n / per_page);
            continue;
        }
        var rn = parseInt(inp);
        if (!isNaN(rn) && rn >= 1 && rn <= n) return rn - 1;
        sy_msg("Invalid round number", true);
    }
}

// Browse/edit rounds and categories
function qe_browse() {
    var ri = qe_pick_round("SELECT ROUND TO BROWSE");
    if (ri < 0) return;

    var cat_running = true;
    while (cat_running) {
        var cats = EXT_ROUNDS[ri].categories || [];
        var b = sy_box("ROUND " + (ri+1) + " - " + cats.length + " CATEGORIES", cats.length + 3, 70);
        var y  = b.by + 3;
        var bw = b.bw;

        for (var ci = 0; ci < Math.min(cats.length, 18); ci++) {
            var key2 = String(ci + 1);
            sy_row(b.bx, y++, bw, key2.length > 1 ? key2 : key2,
                   cats[ci].name, cats[ci].clues.length + " clues", b.bg, b.fg);
        }
        sy_row(b.bx, y++, bw, "D", "Delete a category from this round", "", b.bg, b.fg);
        sy_row(b.bx, y++, bw, "Q", "Back", "", b.bg, b.fg);

        var k = sy_close(b.bx, y, bw, b.fg, b.bg);
        if (k === "Q" || k === "q") {
            cat_running = false;
        } else if (k === "D" || k === "d") {
            qe_delete_category(ri);
        } else {
            var ci2 = parseInt(k) - 1;
            if (!isNaN(ci2) && ci2 >= 0 && ci2 < cats.length) {
                qe_edit_category(ri, ci2);
            }
        }
    }
}

// Edit a single category's clues
function qe_edit_category(ri, ci) {
    var cat = EXT_ROUNDS[ri].categories[ci];
    var clue_running = true;
    while (clue_running) {
        var b = sy_box("EDIT: " + cat.name, cat.clues.length + 4, 70);
        var y = b.by + 3;
        var bw = b.bw;

        // Show category name edit
        sy_row(b.bx, y++, bw, "N", "Category Name", cat.name, b.bg, b.fg);
        gotoxy(b.bx, y++);
        console.write(clr(b.fg, b.bg, true) + B.ML + rep(B.H, bw-2) + B.MR);

        for (var i = 0; i < cat.clues.length; i++) {
            var val  = "$" + (CFG.values[i] || (i+1)*200);
            var clue = cat.clues[i];
            var preview = clue.q.substring(0, 20) + "..  A:" + clue.a.substring(0, 12);
            sy_row(b.bx, y++, bw, String(i+1), val + " Clue", preview, b.bg, b.fg);
        }
        sy_row(b.bx, y++, bw, "Q", "Back (auto-saves)", "", b.bg, b.fg);

        var k = sy_close(b.bx, y, bw, b.fg, b.bg);
        if (k === "N" || k === "n") {
            var nn = sy_input("New category name: ", cat.name, 40);
            if (nn) { cat.name = nn.toUpperCase(); }
        } else if (k === "Q" || k === "q") {
            clue_running = false;
        } else {
            var idx = parseInt(k) - 1;
            if (!isNaN(idx) && idx >= 0 && idx < cat.clues.length) {
                qe_edit_clue(cat, idx);
            }
        }
    }
    // Auto-save on exit
    if (save_external_questions()) {
        rebuild_question_rounds();
        sy_msg("Saved.");
    } else {
        sy_msg("ERROR saving questions.json", true);
    }
}

// Edit a single clue
function qe_edit_clue(cat, idx) {
    var clue = cat.clues[idx];
    var val  = "$" + (CFG.values[idx] || (idx+1)*200);
    clrscr();
    var bw = 70;
    var bx = Math.floor((80-bw)/2)+1;
    var bg = 0; var fg = 5;

    gotoxy(bx, 3);
    console.write(clr(fg,bg,true) + B.TL + rep(B.H, bw-2) + B.TR);
    gotoxy(bx, 4);
    console.write(B.V + clr(0,fg,true) + pad_center("EDIT CLUE  " + val + "  [" + cat.name + "]", bw-2) +
                  clr(fg,bg,true) + B.V);
    gotoxy(bx, 5);
    console.write(B.ML + rep(B.H, bw-2) + B.MR);

    // Show current Q and A
    var qlines = word_wrap(clue.q, bw - 6);
    gotoxy(bx, 6);
    console.write(B.V + clr(3,bg,true) + " QUESTION:" + rep(" ", bw-11) + clr(fg,bg,true) + B.V);
    for (var li = 0; li < Math.min(qlines.length, 4); li++) {
        gotoxy(bx, 7 + li);
        console.write(B.V + clr(7,bg,false) + "  " + pad_right(qlines[li], bw-4) + clr(fg,bg,true) + B.V);
    }
    for (var fi = Math.min(qlines.length,4); fi < 4; fi++) {
        gotoxy(bx, 7 + fi);
        console.write(B.V + rep(" ", bw-2) + B.V);
    }
    gotoxy(bx, 11);
    console.write(B.ML + rep(B.H, bw-2) + B.MR);
    gotoxy(bx, 12);
    console.write(B.V + clr(3,bg,true) + " ANSWER: " + clr(7,bg,false) +
                  pad_right(clue.a, bw-11) + clr(fg,bg,true) + B.V);
    gotoxy(bx, 13);
    console.write(B.ML + rep(B.H, bw-2) + B.MR);
    gotoxy(bx, 14);
    console.write(B.V + clr(7,bg,false) +
        pad_center("[Q] Edit Question   [A] Edit Answer   [X] Cancel", bw-2) +
        clr(fg,bg,true) + B.V);
    gotoxy(bx, 15);
    console.write(B.BL + rep(B.H, bw-2) + B.BR);

    gotoxy(bx+2, 17);
    console.write(clr(3,0,true) + "Choice: ");
    show_cursor();
    var k = console.getkey();
    hide_cursor();

    if (k === "Q" || k === "q") {
        var nq = sy_input("New question text: ", clue.q, 200);
        if (nq && nq !== clue.q) clue.q = nq;
    } else if (k === "A" || k === "a") {
        var na = sy_input("New answer: ", clue.a, 80);
        if (na && na !== clue.a) clue.a = na;
    }
}

// Add a new category to an existing round
function qe_add_category() {
    var ri = qe_pick_round("SELECT ROUND TO ADD CATEGORY TO");
    if (ri < 0) return;

    clrscr();
    gotoxy(1, 5);
    console.write(clr(5, 0, true) + pad_center("ADD NEW CATEGORY TO ROUND " + (ri+1), 80));

    var cname = sy_input("Category name (UPPERCASE recommended): ", "", 40);
    if (!cname) return;
    cname = cname.toUpperCase();

    var new_cat = { name: cname, clues: [] };
    for (var i = 0; i < CFG.values.length; i++) {
        gotoxy(1, 8 + i * 2);
        console.write(clr(3, 0, true) + "  $" + CFG.values[i] + " CLUE:");
        var q = sy_input("  Question: ", "", 200);
        if (!q) { sy_msg("Cancelled.", true); return; }
        var a = sy_input("  Answer:   ", "", 80);
        if (!a) { sy_msg("Cancelled.", true); return; }
        new_cat.clues.push({ q: q, a: a });
    }

    if (!EXT_ROUNDS[ri].categories) EXT_ROUNDS[ri].categories = [];
    EXT_ROUNDS[ri].categories.push(new_cat);

    if (save_external_questions()) {
        rebuild_question_rounds();
        sy_msg("Category \"" + cname + "\" added to round " + (ri+1) + " and saved.");
    } else {
        sy_msg("ERROR saving questions.json", true);
    }
}

// Add a whole new round
function qe_add_round() {
    clrscr();
    gotoxy(1, 5);
    console.write(clr(5, 0, true) + pad_center("ADD NEW ROUND  (" + CFG.num_categories + " categories)", 80));
    gotoxy(1, 7);
    console.write(clr(7, 0, false) + pad_center("You will enter " + CFG.num_categories + " categories, each with " +
                  CFG.values.length + " clues.", 80));
    gotoxy(1, 9);
    console.write(pad_center("Press ENTER to begin or Q to cancel: ", 80));
    show_cursor();
    var k = console.getkey();
    hide_cursor();
    if (k === "Q" || k === "q") return;

    var new_round = { categories: [] };
    for (var ci = 0; ci < CFG.num_categories; ci++) {
        clrscr();
        gotoxy(1, 3);
        console.write(clr(5,0,true) + pad_center("NEW ROUND - CATEGORY " + (ci+1) + " of " + CFG.num_categories, 80));

        var cname2 = sy_input("Category name: ", "", 40);
        if (!cname2) { sy_msg("Cancelled.", true); return; }
        var new_cat2 = { name: cname2.toUpperCase(), clues: [] };

        for (var vi = 0; vi < CFG.values.length; vi++) {
            gotoxy(1, 6 + vi * 3);
            console.write(clr(3,0,true) + "  Clue " + (vi+1) + " [$" + CFG.values[vi] + "]:");
            var q2 = sy_input("  Question: ", "", 200);
            if (!q2) { sy_msg("Cancelled.", true); return; }
            var a2 = sy_input("  Answer:   ", "", 80);
            if (!a2) { sy_msg("Cancelled.", true); return; }
            new_cat2.clues.push({ q: q2, a: a2 });
        }
        new_round.categories.push(new_cat2);
    }

    EXT_ROUNDS.push(new_round);
    if (save_external_questions()) {
        rebuild_question_rounds();
        sy_msg("New round added! Now " + EXT_ROUNDS.length + " external rounds. Saved.");
    } else {
        sy_msg("ERROR saving questions.json", true);
    }
}

// Delete a round
function qe_delete_round() {
    var ri = qe_pick_round("SELECT ROUND TO DELETE");
    if (ri < 0) return;

    clrscr();
    var cats = (EXT_ROUNDS[ri].categories || []);
    gotoxy(1, 8);
    console.write(clr(1,0,true) + pad_center(
        "DELETE Round " + (ri+1) + "? (" + cats.length + " categories)  [Y/N]", 80));
    show_cursor();
    var k = console.getkey();
    hide_cursor();
    if (k === "Y" || k === "y") {
        EXT_ROUNDS.splice(ri, 1);
        if (save_external_questions()) {
            rebuild_question_rounds();
            sy_msg("Round deleted. " + EXT_ROUNDS.length + " external rounds remain.");
        } else {
            sy_msg("ERROR saving", true);
        }
    } else {
        sy_msg("Cancelled.");
    }
}

// Delete a category from a round
function qe_delete_category(ri) {
    var cats = EXT_ROUNDS[ri].categories || [];
    if (cats.length === 0) { sy_msg("No categories in this round.", true); return; }

    var b = sy_box("DELETE CATEGORY FROM ROUND " + (ri+1), cats.length + 2, 68);
    var y = b.by + 3;
    var bw = b.bw;
    for (var ci = 0; ci < Math.min(cats.length, 18); ci++) {
        sy_row(b.bx, y++, bw, String(ci+1), cats[ci].name, cats[ci].clues.length + " clues", b.bg, b.fg);
    }
    sy_row(b.bx, y++, bw, "Q", "Cancel", "", b.bg, b.fg);

    var k = sy_close(b.bx, y, bw, b.fg, b.bg);
    if (k === "Q" || k === "q") return;
    var ci2 = parseInt(k) - 1;
    if (!isNaN(ci2) && ci2 >= 0 && ci2 < cats.length) {
        var cname = cats[ci2].name;
        cats.splice(ci2, 1);
        if (save_external_questions()) {
            rebuild_question_rounds();
            sy_msg("Deleted \"" + cname + "\". Saved.");
        } else {
            sy_msg("ERROR saving", true);
        }
    }
}

// =============================================================================
//  SYSOP DIAGNOSTICS
// =============================================================================
function sysop_diagnostics() {
    clrscr();
    var bg = 0; var fg = 5;
    var bw = 76; var bx = Math.floor((80-bw)/2)+1;

    gotoxy(bx, 1);
    console.write(clr(fg,bg,true) + B.TL + rep(B.H,bw-2) + B.TR);
    gotoxy(bx, 2);
    console.write(B.V + clr(0,fg,true) + pad_center(" JEOPARDY! SYSOP DIAGNOSTICS ", bw-2) +
                  clr(fg,bg,true) + B.V);
    gotoxy(bx, 3);
    console.write(B.ML + rep(B.H,bw-2) + B.MR);

    var y = 4;
    function drow(label, value, ok) {
        gotoxy(bx, y++);
        var vc = (ok === true) ? clr(2,bg,true) : (ok === false) ? clr(1,bg,true) : clr(3,bg,false);
        console.write(clr(fg,bg,true) + B.V +
            clr(7,bg,false)  + " " + pad_right(label, 26) +
            vc               + pad_right(String(value), bw-30) + " " +
            clr(fg,bg,true)  + B.V);
    }
    function dsep() {
        gotoxy(bx, y++);
        console.write(clr(fg,bg,true) + B.ML + rep(B.H,bw-2) + B.MR);
    }

    // Script location
    drow("GAME_DIR", GAME_DIR, null);
    dsep();

    // INI file
    var ini_path = GAME_DIR + "jeopardy.ini";
    var ini_f = new File(ini_path);
    var ini_ok = ini_f.open("r");
    if (ini_ok) ini_f.close();
    drow("jeopardy.ini path", ini_path, ini_ok);
    dsep();

    // Questions file
    drow("CFG.questions_file", CFG.questions_file, null);
    var q_f = new File(CFG.questions_file);
    var q_ok = q_f.open("r");
    if (q_ok) {
        q_f.close();
        drow("questions.json EXISTS", "YES - file opened successfully", true);
    } else {
        drow("questions.json EXISTS", "NO - could not open file", false);
        // Try forward-slash version
        var alt = CFG.questions_file.replace(/\\/g, "/");
        var a_f = new File(alt);
        var a_ok = a_f.open("r");
        if (a_ok) { a_f.close(); }
        drow("  (forward slash alt)", alt, a_ok);
    }
    drow("EXT_ROUNDS loaded", EXT_ROUNDS.length + " rounds", EXT_ROUNDS.length > 0);
    dsep();

    // Score file
    drow("CFG.score_file", CFG.score_file, null);
    var s_f = new File(CFG.score_file);
    var s_ok = s_f.open("r");
    if (s_ok) s_f.close();
    else { s_ok = s_f.open("w"); if (s_ok) s_f.close(); }
    drow("score file writable", s_ok ? "YES" : "NO - check permissions", s_ok);
    dsep();

    // Rounds summary
    drow("Built-in rounds", CFG.use_builtin ? BUILTIN_ROUNDS.length + " (ENABLED)" : "DISABLED", CFG.use_builtin);
    drow("External rounds", EXT_ROUNDS.length, EXT_ROUNDS.length > 0);
    drow("TOTAL active rounds", QUESTION_ROUNDS.length, QUESTION_ROUNDS.length > 0);
    dsep();

    // Hint mode
    var hm = ["0: None (default)", "1: Underscores Only", "2: Letters + Underscores"];
    drow("Hint mode", hm[CFG.hint_mode] || CFG.hint_mode, null);
    drow("Sysop level required", CFG.sysop_level, null);

    gotoxy(bx, y++);
    console.write(clr(fg,bg,true) + B.BL + rep(B.H,bw-2) + B.BR);
    gotoxy(bx, y+1);
    console.write(clr(7,0,false) + pad_center("Press any key to continue...", bw));
    console.getkey();
}

// =============================================================================
//  MAIN ENTRY POINT
// =============================================================================
function main() {
    load_config();
    // If built-in questions are disabled, clear them before loading external
    if (!CFG.use_builtin) {
        QUESTION_ROUNDS = [];
    }
    try_load_external_questions();
    hide_cursor();
    calc_rows();

    var player_name = "Player";
    try {
        if (typeof user !== "undefined" && user && user.alias) {
            player_name = user.alias;
        }
    } catch (e) {}

    var running       = true;
    var round_counter = 0;

    while (running) {
        var choice = show_main_menu();

        if (choice === "1") {
            // Guard: no rounds available
            if (QUESTION_ROUNDS.length === 0) {
                clrscr();
                gotoxy(1, 8);
                console.write(clr(1, 0, true) + pad_center("NO QUESTIONS AVAILABLE!", 80));
                gotoxy(1, 10);
                console.write(clr(7, 0, false) + pad_center("Built-in questions are disabled and no questions.json was loaded.", 80));
                gotoxy(1, 11);
                console.write(pad_center("Configured path: " + CFG.questions_file, 80));
                gotoxy(1, 13);
                console.write(clr(3, 0, true) + pad_center("TIPS:", 80));
                gotoxy(1, 14);
                console.write(clr(7, 0, false) + pad_center("1. Use forward slashes in jeopardy.ini: C:/sbbs/xtrn/jeopardy/questions.json", 80));
                gotoxy(1, 15);
                console.write(pad_center("2. Make sure questions.json exists at that path.", 80));
                gotoxy(1, 16);
                console.write(pad_center("3. Enable built-in questions via Sysop Menu if needed.", 80));
                gotoxy(1, 17);
                console.write(pad_center("4. Use Sysop Menu -> Diagnostics to check all paths.", 80));
                gotoxy(1, 19);
                console.write(pad_center("Press any key to return to the menu...", 80));
                console.getkey();
                continue;
            }

            // Player setup
            var players = setup_players(player_name);

            // Pick next round
            var ri = round_counter % QUESTION_ROUNDS.length;
            round_counter++;

            var _rd   = QUESTION_ROUNDS[ri];
            var _cats = Array.isArray(_rd) ? _rd : _rd.categories;

            // Shuffle categories
            var round_copy = { categories: _cats.slice() };
            for (var si = round_copy.categories.length - 1; si > 0; si--) {
                var sj  = Math.floor(Math.random() * (si + 1));
                var tmp = round_copy.categories[si];
                round_copy.categories[si] = round_copy.categories[sj];
                round_copy.categories[sj] = tmp;
            }

            var final_players = play_round(round_copy, players);
            show_final_score(final_players);

        } else if (choice === "2") {
            show_high_scores();
        } else if (choice === "3") {
            show_instructions();
        } else if (choice === "S" || choice === "s") {
            // Sysop menu - re-verify level in case menu was shown by error
            var ok_sysop = false;
            try {
                if (typeof user !== "undefined" && user &&
                    typeof user.security !== "undefined" &&
                    user.security.level >= CFG.sysop_level) {
                    ok_sysop = true;
                }
            } catch(e) {}
            if (ok_sysop) {
                sysop_menu();
            }
        } else if (choice === "Q" || choice === "q") {
            running = false;
        }
    }

    // Clean exit
    show_cursor();
    clrscr();
    console.write(RESET);
    console.writeln(clr(7, 0, false) + "Thank you for playing Jeopardy! BBS Edition!");
    console.writeln("");
}

main();