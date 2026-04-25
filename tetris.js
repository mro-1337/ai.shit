// ===== START PART 1 =====
// Synchronet BBS Tetris Game - tetris.js

load("sbbsdefs.js");

const MAX_PT_Y = 5;
const MAX_PT_X = 5;
const MAX_SHAPE = 7;
const MAX_LEVEL = 18;
const MAX_HIGH_SCORES = 10;
const MAX_SCREEN_ROW = 23;  // Maximum screen row - nothing should go below this
const GAME_DIR = system.exec_dir + "../xtrn/tetrisjs/";
const CONFIG_FILE = GAME_DIR + "tetris.cfg";
//const SCORES_FILE = system.data_dir + "tetris.scores";
const SCORES_FILE = GAME_DIR + "tetris.scores";

const BLOCK_STYLES = [
    { name: "Brackets  [ ]", filled: "[]",       empty: "  " },
    { name: "Solid     \xDB\xDB",  filled: "\xDB\xDB", empty: "  " },
    { name: "Light     \xB0\xB0",  filled: "\xB0\xB0", empty: "  " },
    { name: "Medium    \xB1\xB1",  filled: "\xB1\xB1", empty: "  " },
    { name: "Dark      \xB2\xB2",  filled: "\xB2\xB2", empty: "  " },
    { name: "Half Top  \xDF\xDF",  filled: "\xDF\xDF", empty: "  " },
    { name: "Half Bot  \xDC\xDC",  filled: "\xDC\xDC", empty: "  " },
    { name: "Hash      ##", filled: "##",         empty: "  " },
    { name: "Dots      \xFA\xFA",  filled: "\xFA\xFA", empty: "  " },
    { name: "Stars     **", filled: "**",         empty: "  " },
    { name: "Smiles    \x01\x01",  filled: "\x01\x01", empty: "  " },
    { name: "Arrows    \x10\x10",  filled: "\x10\x10", empty: "  " }
];

const SHAPE_COLORS = [
    "\x1b[1;36;44m",   // I - cyan on blue
    "\x1b[1;34;40m",   // J - blue on black
    "\x1b[1;33;40m",   // L - yellow on black
    "\x1b[1;35;40m",   // T - magenta on black
    "\x1b[1;33;43m",   // O - yellow on yellow bg
    "\x1b[1;32;40m",   // S - green on black
    "\x1b[1;31;40m"    // Z - red on black
];

const PLACED_COLOR = "\x1b[0;37;40m";

const SHAPES = [
    // I-piece
    [
        [[0,0,0,0,0],[0,0,0,0,0],[0,1,1,1,1],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,0,1,0,0],[0,0,1,0,0],[0,0,1,0,0],[0,0,1,0,0],[0,0,0,0,0]],
        [[0,0,0,0,0],[0,0,0,0,0],[1,1,1,1,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,0,1,0,0],[0,0,1,0,0],[0,0,1,0,0],[0,0,1,0,0],[0,0,0,0,0]]
    ],
    // J-piece
    [
        [[0,1,0,0,0],[0,1,1,1,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,1,1,0,0],[0,1,0,0,0],[0,1,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,1,1,1,0],[0,0,0,1,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,0,1,0,0],[0,0,1,0,0],[0,1,1,0,0],[0,0,0,0,0],[0,0,0,0,0]]
    ],
    // L-piece
    [
        [[0,0,0,1,0],[0,1,1,1,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,1,0,0,0],[0,1,0,0,0],[0,1,1,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,1,1,1,0],[0,1,0,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,1,1,0,0],[0,0,1,0,0],[0,0,1,0,0],[0,0,0,0,0],[0,0,0,0,0]]
    ],
    // T-piece
    [
        [[0,1,1,1,0],[0,0,1,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,1,0,0,0],[0,1,1,0,0],[0,1,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,0,1,0,0],[0,1,1,1,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,0,1,0,0],[0,1,1,0,0],[0,0,1,0,0],[0,0,0,0,0],[0,0,0,0,0]]
    ],
    // O-piece
    [
        [[0,1,1,0,0],[0,1,1,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,1,1,0,0],[0,1,1,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,1,1,0,0],[0,1,1,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,1,1,0,0],[0,1,1,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]]
    ],
    // S-piece
    [
        [[0,0,1,1,0],[0,1,1,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,1,0,0,0],[0,1,1,0,0],[0,0,1,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,0,1,1,0],[0,1,1,0,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,1,0,0,0],[0,1,1,0,0],[0,0,1,0,0],[0,0,0,0,0],[0,0,0,0,0]]
    ],
    // Z-piece
    [
        [[0,1,1,0,0],[0,0,1,1,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,0,1,0,0],[0,1,1,0,0],[0,1,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,1,1,0,0],[0,0,1,1,0],[0,0,0,0,0],[0,0,0,0,0],[0,0,0,0,0]],
        [[0,0,1,0,0],[0,1,1,0,0],[0,1,0,0,0],[0,0,0,0,0],[0,0,0,0,0]]
    ]
];

const SHAPE_POINTS = [4, 6, 6, 5, 6, 5, 5];

const LEVELS = [
    {delay: 800,  lines: 0},
    {delay: 700,  lines: 10},
    {delay: 600,  lines: 20},
    {delay: 500,  lines: 30},
    {delay: 450,  lines: 40},
    {delay: 400,  lines: 50},
    {delay: 350,  lines: 60},
    {delay: 300,  lines: 70},
    {delay: 260,  lines: 80},
    {delay: 220,  lines: 90},
    {delay: 190,  lines: 100},
    {delay: 160,  lines: 110},
    {delay: 130,  lines: 120},
    {delay: 100,  lines: 130},
    {delay: 80,   lines: 150},
    {delay: 60,   lines: 160},
    {delay: 40,   lines: 175},
    {delay: 25,   lines: 200}
];

var cfg = {
    blockStyle:        0,
    speedMultiplier:   100,
    startLevel:        1,
    useAnsiBackground: true,
    coloredPieces:     true,
    gridWidth:         15,
    gridHeight:        19  // Max safe height: MAX_SCREEN_ROW (23) - gridY (3) - 1 = 19
};

function loadConfig() {
    var f = new File(CONFIG_FILE);
    if (f.open("r")) {
        var data = f.read();
        f.close();
        if (data) {
            try {
                var saved = JSON.parse(data);
                for (var k in saved) {
                    if (cfg.hasOwnProperty(k)) cfg[k] = saved[k];
                }
            } catch(e) {}
        }
    }
}

function saveConfig() {
    var f = new File(CONFIG_FILE);
    if (f.open("w")) {
        f.write(JSON.stringify(cfg));
        f.close();
    }
}

function getLevelDelay(level) {
    var base = LEVELS[level - 1].delay;
    return Math.floor(base * 100 / cfg.speedMultiplier);
}

// ===== END PART 1 =====
// ===== START PART 2 =====

var ATTR = {
    reset:    "\x1b[0m",
    border:   "\x1b[1;37;40m",
    text:     "\x1b[1;37;40m",
    title:    "\x1b[1;33;40m",
    subtitle: "\x1b[1;36;40m",
    hilight:  "\x1b[1;32;40m",
    gameover: "\x1b[1;31;40m",
    paused:   "\x1b[1;36;40m",
    preview:  "\x1b[1;35;40m",
    menusel:  "\x1b[1;30;47m",
    menuitem: "\x1b[0;37;40m",
    empty:    "\x1b[0;30;40m"
};

var game = {
    grid:            null,
    gridWidth:       10,
    gridHeight:      20,
    gridX:           10,
    gridY:           3,
    currentShape:    0,
    currentRotation: 0,
    blockX:          0,
    blockY:          0,
    nextShape:       0,
    score:           0,
    lines:           0,
    level:           1,
    lastMove:        0,
    gameOver:        false,
    paused:          false
};

function initGrid() {
    game.gridWidth = cfg.gridWidth;
    game.gridHeight = cfg.gridHeight;
    
    // Ensure grid doesn't exceed MAX_SCREEN_ROW
    // gridY + gridHeight + 1 (bottom border) <= MAX_SCREEN_ROW
    var maxAllowedHeight = MAX_SCREEN_ROW - game.gridY - 1;
    if (game.gridHeight > maxAllowedHeight) {
        game.gridHeight = maxAllowedHeight;
    }
    
    game.grid = [];
    for (var y = 0; y < game.gridHeight; y++) {
        game.grid[y] = [];
        for (var x = 0; x < game.gridWidth; x++) {
            game.grid[y][x] = 0;
        }
    }
}

function writeAt(col, row, attr, str) {
    console.gotoxy(col, row);
    console.print(attr + str);
}

function stripAnsi(str) {
    // Remove ANSI color codes and Synchronet color codes to get visible length
    return str.replace(/\x1b\[[^m]*m/g, '').replace(/\x01[hncrgybmw]/gi, '');
}

function getVisibleLength(str) {
    return stripAnsi(str).length;
}

function loadAnsiBackground() {
    if (!cfg.useAnsiBackground) return;
    var files = directory(GAME_DIR + "/ansi/*.ans");
    if (!files || files.length === 0) return;
    var pick = files[Math.floor(Math.random() * files.length)];
    var f = new File(pick);
    if (f.open("rb")) {
        var raw = f.read();
        f.close();
        if (raw) {
            console.gotoxy(1, 1);
            console.print(raw);
        }
    }
}

function getBlockChars(filled, isActive, shapeIndex) {
    var style = BLOCK_STYLES[cfg.blockStyle];
    if (!filled) {
        // Always use black background with spaces to cover ANSI art
        return { attr: "\x1b[0;40m", ch: "  " };
    }
    if (cfg.coloredPieces && shapeIndex >= 0 && shapeIndex < MAX_SHAPE) {
        return { attr: SHAPE_COLORS[shapeIndex], ch: style.filled };
    }
    return { attr: PLACED_COLOR, ch: style.filled };
}

function fillGridWithBlack() {
    // Fill the entire play area with black background to cover ANSI art
    var blackBg = "\x1b[0;40m";
    for (var gy = 0; gy < game.gridHeight; gy++) {
        for (var gx = 0; gx < game.gridWidth; gx++) {
            writeAt(game.gridX + gx * 2, game.gridY + gy, blackBg, "  ");
        }
    }
}

function renderGrid() {
    for (var gy = 0; gy < game.gridHeight; gy++) {
        for (var gx = 0; gx < game.gridWidth; gx++) {
            var val = game.grid[gy][gx];
            var bc  = getBlockChars(val !== 0, false, val - 1);
            writeAt(game.gridX + gx * 2, game.gridY + gy, bc.attr, bc.ch);
        }
    }
}

function renderPiece(erase) {
    var shape = SHAPES[game.currentShape][game.currentRotation];
    for (var py = 0; py < MAX_PT_Y; py++) {
        for (var px = 0; px < MAX_PT_X; px++) {
            if (shape[py][px] !== 0) {
                var gx = game.blockX + px;
                var gy = game.blockY + py;
                if (gx >= 0 && gx < game.gridWidth &&
                    gy >= 0 && gy < game.gridHeight) {
                    if (erase) {
                        var val = game.grid[gy][gx];
                        var bc  = getBlockChars(val !== 0, false, val - 1);
                        writeAt(game.gridX + gx * 2, game.gridY + gy,
                                bc.attr, bc.ch);
                    } else {
                        var bc = getBlockChars(true, true, game.currentShape);
                        writeAt(game.gridX + gx * 2, game.gridY + gy,
                                bc.attr, bc.ch);
                    }
                }
            }
        }
    }
}

function drawBorder() {
    var left   = game.gridX - 1;
    var right  = game.gridX + game.gridWidth * 2;
    var top    = game.gridY - 1;
    var bottom = game.gridY + game.gridHeight;

    console.print(ATTR.border);

    // Top
    console.gotoxy(left, top);
    console.print("\xDA");
    for (var i = 0; i < game.gridWidth * 2; i++) console.print("\xC4");
    console.print("\xBF");

    // Sides
    for (var row = game.gridY; row < game.gridY + game.gridHeight; row++) {
        console.gotoxy(left,  row); console.print("\xB3");
        console.gotoxy(right, row); console.print("\xB3");
    }

    // Bottom
    console.gotoxy(left, bottom);
    console.print("\xC0");
    for (var i = 0; i < game.gridWidth * 2; i++) console.print("\xC4");
    console.print("\xD9");
}

function drawInfoPanel() {
    // Draw a bordered panel on the right side with black background
    var panelLeft = game.gridX + game.gridWidth * 2 + 2;
    var panelWidth = 20;
    var panelTop = game.gridY - 1;
    
    // Calculate panel height to not exceed MAX_SCREEN_ROW
    var maxPanelHeight = MAX_SCREEN_ROW - panelTop;
    var panelHeight = Math.min(23, maxPanelHeight);

    console.print(ATTR.border);

    // Top border
    console.gotoxy(panelLeft, panelTop);
    console.print("\xDA");
    for (var i = 0; i < panelWidth; i++) console.print("\xC4");
    console.print("\xBF");

    // Fill with black background and side borders
    for (var row = panelTop + 1; row < panelTop + panelHeight; row++) {
        console.gotoxy(panelLeft, row);
        console.print("\xB3");
        // Fill inside with black
        console.print("\x1b[0;30;40m");
        for (var i = 0; i < panelWidth; i++) console.print(" ");
        console.print(ATTR.border);
        console.print("\xB3");
    }

    // Bottom border
    console.gotoxy(panelLeft, panelTop + panelHeight);
    console.print("\xC0");
    for (var i = 0; i < panelWidth; i++) console.print("\xC4");
    console.print("\xD9");
}


// ===== END PART 2 =====

// ===== START PART 3 =====

function canPlace(bx, by, rotation) {
    var shape = SHAPES[game.currentShape][rotation];
    for (var py = 0; py < MAX_PT_Y; py++) {
        for (var px = 0; px < MAX_PT_X; px++) {
            if (shape[py][px] !== 0) {
                var gx = bx + px;
                var gy = by + py;
                if (gx < 0 || gx >= game.gridWidth ||
                    gy >= game.gridHeight) {
                    return false;
                }
                if (gy >= 0 && game.grid[gy][gx] !== 0) {
                    return false;
                }
            }
        }
    }
    return true;
}

function lockPiece() {
    var shape = SHAPES[game.currentShape][game.currentRotation];
    for (var py = 0; py < MAX_PT_Y; py++) {
        for (var px = 0; px < MAX_PT_X; px++) {
            if (shape[py][px] !== 0) {
                var gx = game.blockX + px;
                var gy = game.blockY + py;
                if (gx >= 0 && gx < game.gridWidth &&
                    gy >= 0 && gy < game.gridHeight) {
                    game.grid[gy][gx] = game.currentShape + 1;
                }
            }
        }
    }
}

function checkLines() {
    game.score += SHAPE_POINTS[game.currentShape];

    var linesCleared = 0;
    var y = game.gridHeight - 1;

    while (y >= 0) {
        var complete = true;
        for (var x = 0; x < game.gridWidth; x++) {
            if (game.grid[y][x] === 0) {
                complete = false;
                break;
            }
        }

        if (complete) {
            linesCleared++;
            game.lines++;

            for (var dy = y; dy > 0; dy--) {
                for (var x = 0; x < game.gridWidth; x++) {
                    game.grid[dy][x] = game.grid[dy - 1][x];
                }
            }
            for (var x = 0; x < game.gridWidth; x++) {
                game.grid[0][x] = 0;
            }
            // do NOT decrement y - recheck same row after shift
        } else {
            y--;
        }
    }

    if (linesCleared > 0) {
        var bonus = [0, 100, 300, 500, 800];
        game.score += bonus[Math.min(linesCleared, 4)] * game.level;
        renderGrid();
        console.print("\x07");

        if (game.level < MAX_LEVEL &&
            game.lines >= LEVELS[game.level].lines) {
            game.level++;
            console.print("\x07\x07");
        }
    }

    updateStats();
}

function moveDown() {
    if (canPlace(game.blockX, game.blockY + 1, game.currentRotation)) {
        renderPiece(true);
        game.blockY++;
        renderPiece(false);
        return true;
    } else {
        renderPiece(false);
        lockPiece();
        checkLines();
        newBlock();
        return false;
    }
}

function fastDrop() {
    renderPiece(true);
    while (canPlace(game.blockX, game.blockY + 1, game.currentRotation)) {
        game.blockY++;
    }
    renderPiece(false);
    lockPiece();
    checkLines();
    newBlock();
}

function moveLeft() {
    if (canPlace(game.blockX - 1, game.blockY, game.currentRotation)) {
        renderPiece(true);
        game.blockX--;
        renderPiece(false);
    }
}

function moveRight() {
    if (canPlace(game.blockX + 1, game.blockY, game.currentRotation)) {
        renderPiece(true);
        game.blockX++;
        renderPiece(false);
    }
}

function rotatePiece(dir) {
    var newRot = (game.currentRotation + dir + 4) % 4;
    if (canPlace(game.blockX, game.blockY, newRot)) {
        renderPiece(true);
        game.currentRotation = newRot;
        renderPiece(false);
    }
}

function newBlock() {
    game.currentShape    = game.nextShape;
    game.nextShape       = Math.floor(Math.random() * MAX_SHAPE);
    game.currentRotation = Math.floor(Math.random() * 4);
    game.blockX          = Math.floor(game.gridWidth / 2) - 2;
    game.blockY          = 0;
    game.lastMove        = Date.now();

    if (!canPlace(game.blockX, game.blockY, game.currentRotation)) {
        game.gameOver = true;
    } else {
        renderPiece(false);
        drawNextShape();
    }
}

function drawNextShape() {
    var px = game.gridX + game.gridWidth * 2 + 4;
    var py = game.gridY + 1;

    writeAt(px, py - 1, ATTR.text, "NEXT:          ");

    for (var y = 0; y < 5; y++) {
        writeAt(px, py + y, "\x1b[0;30;40m", "          ");
    }

    var shape = SHAPES[game.nextShape][0];
    for (var sy = 0; sy < MAX_PT_Y; sy++) {
        for (var sx = 0; sx < MAX_PT_X; sx++) {
            if (shape[sy][sx] !== 0) {
                var bc = getBlockChars(true, true, game.nextShape);
                writeAt(px + sx * 2, py + sy, bc.attr, bc.ch);
            }
        }
    }
}

function updateStats() {
    var sx = game.gridX + game.gridWidth * 2 + 4;
    var sy = game.gridY + 4;

    writeAt(sx, sy,     ATTR.text, "LEVEL: " + game.level + "      ");
    writeAt(sx, sy + 1, ATTR.text, "LINES: " + game.lines + "      ");
    writeAt(sx, sy + 2, ATTR.text, "SCORE: " + game.score + "      ");
    writeAt(sx, sy + 3, ATTR.text, "SPEED: " + cfg.speedMultiplier + "%   ");
}

function drawInstructions() {
    var ix = game.gridX + game.gridWidth * 2 + 4;
    var iy = game.gridY + 9;

    writeAt(ix, iy,     ATTR.subtitle, "CONTROLS:         ");
    writeAt(ix, iy + 1, ATTR.text,     "UP    : Rotate CW ");
    writeAt(ix, iy + 2, ATTR.text,     "Z     : Rotate CCW");
    writeAt(ix, iy + 3, ATTR.text,     "LEFT  : Move Left ");
    writeAt(ix, iy + 4, ATTR.text,     "RIGHT : Move Right");
    writeAt(ix, iy + 5, ATTR.text,     "DOWN  : Fast Drop ");
    writeAt(ix, iy + 6, ATTR.text,     "SPACE : Rotate CW ");
    writeAt(ix, iy + 7, ATTR.text,     "P     : Pause     ");
    writeAt(ix, iy + 8, ATTR.text,     "Q/ESC : Quit      ");
}

function loadScores() {
    var scores = [];
    var f = new File(SCORES_FILE);
    if (f.open("r")) {
        var data = f.read();
        f.close();
        if (data) {
            try { scores = JSON.parse(data); } catch(e) { scores = []; }
        }
    }
    return scores;
}

function saveHighScore() {
    var scores = loadScores();
    scores.push({
        name:  user.alias,
        score: game.score,
        lines: game.lines,
        level: game.level,
        date:  new Date().toISOString()
    });
    scores.sort(function(a, b) { return b.score - a.score; });
    if (scores.length > MAX_HIGH_SCORES) {
        scores = scores.slice(0, MAX_HIGH_SCORES);
    }
    var f = new File(SCORES_FILE);
    if (f.open("w")) {
        f.write(JSON.stringify(scores));
        f.close();
    }
}

function showHighScores() {
    console.clear();
    console.print(ATTR.title);
    console.print("\r\n\r\n");
    console.print("  \xC9\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xBB\r\n");
    console.print("  \xBA          TETRIS HIGH SCORES                     \xBA\r\n");
    console.print("  \xC8\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xBC\r\n\r\n");

    var scores = loadScores();

    if (scores.length === 0) {
        console.print(ATTR.text + "  No scores yet!\r\n");
    } else {
        console.print(ATTR.subtitle);
        console.print("  RANK  NAME                 SCORE      LINES  LEVEL\r\n");
        console.print("  ----  -------------------  ---------  -----  -----\r\n");
        for (var i = 0; i < scores.length; i++) {
            console.print(ATTR.text);
            console.print(format("  %2d    %-19s  %9d  %5d  %5d\r\n",
                i + 1,
                scores[i].name,
                scores[i].score,
                scores[i].lines,
                scores[i].level));
        }
    }
    console.print(ATTR.text + "\r\n  Press any key...\r\n");
    console.getkey();
}

// ===== END PART 3 =====

// ===== START PART 4 =====

function drawSetupMenu() {
    console.clear();
    console.print(ATTR.title);
    console.print("\r\n");
    console.print("  \xC9\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xBB\r\n");
    console.print("  \xBA       TETRIS SETUP MENU      \xBA\r\n");
    console.print("  \xC8\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xBC\r\n\r\n");

    console.print(ATTR.hilight + "  1. " + ATTR.text +
        "Block Style    : " + ATTR.hilight +
        BLOCK_STYLES[cfg.blockStyle].name + "\r\n");

    console.print(ATTR.hilight + "  2. " + ATTR.text +
        "Speed          : " + ATTR.hilight +
        cfg.speedMultiplier + "%\r\n");

    console.print(ATTR.hilight + "  3. " + ATTR.text +
        "Start Level    : " + ATTR.hilight +
        cfg.startLevel + "\r\n");

    console.print(ATTR.hilight + "  4. " + ATTR.text +
        "ANSI Background: " + ATTR.hilight +
        (cfg.useAnsiBackground ? "ON" : "OFF") + "\r\n");

    console.print(ATTR.hilight + "  5. " + ATTR.text +
        "Colored Pieces : " + ATTR.hilight +
        (cfg.coloredPieces ? "ON" : "OFF") + "\r\n");

    console.print(ATTR.hilight + "  6. " + ATTR.text +
        "Grid Width     : " + ATTR.hilight +
        cfg.gridWidth + "\r\n");

    console.print(ATTR.hilight + "  7. " + ATTR.text +
        "Grid Height    : " + ATTR.hilight +
        cfg.gridHeight + "\r\n");

    console.print("\r\n");
    console.print(ATTR.hilight + "  S. " + ATTR.text + "Save Settings\r\n");
    console.print(ATTR.hilight + "  Q. " + ATTR.text + "Back to Title\r\n");
    console.print("\r\n");
    console.print(ATTR.subtitle + "  Choice: " + ATTR.text);
}

function setupBlockStyle() {
    console.clear();
    console.print(ATTR.title + "\r\n  SELECT BLOCK STYLE\r\n\r\n");

    for (var i = 0; i < BLOCK_STYLES.length; i++) {
        var marker = (i === cfg.blockStyle) ? ATTR.menusel : ATTR.text;
        console.print(marker);
        console.print("  " + (i + 1) + ". " +
            BLOCK_STYLES[i].name + "  " +
            BLOCK_STYLES[i].filled + "\r\n");
    }

    console.print(ATTR.text + "\r\n  Press number to select, Q to cancel: ");

    var key = console.getkey();
    var num = parseInt(key);
    if (!isNaN(num) && num >= 1 && num <= BLOCK_STYLES.length) {
        cfg.blockStyle = num - 1;
    }
}

function setupSpeed() {
    console.clear();
    console.print(ATTR.title + "\r\n  SELECT SPEED\r\n\r\n");

    var speeds = [
        { label: "25%  - Very Slow",  val: 25  },
        { label: "50%  - Slow",       val: 50  },
        { label: "75%  - Moderate",   val: 75  },
        { label: "100% - Normal",     val: 100 },
        { label: "125% - Fast",       val: 125 },
        { label: "150% - Faster",     val: 150 },
        { label: "200% - Very Fast",  val: 200 },
        { label: "300% - Insane",     val: 300 }
    ];

    for (var i = 0; i < speeds.length; i++) {
        var marker = (speeds[i].val === cfg.speedMultiplier) ?
            ATTR.menusel : ATTR.text;
        console.print(marker);
        console.print("  " + (i + 1) + ". " + speeds[i].label + "\r\n");
    }

    console.print(ATTR.text + "\r\n  Press number to select, Q to cancel: ");

    var key = console.getkey();
    var num = parseInt(key);
    if (!isNaN(num) && num >= 1 && num <= speeds.length) {
        cfg.speedMultiplier = speeds[num - 1].val;
    }
}

function setupStartLevel() {
    console.clear();
    console.print(ATTR.title + "\r\n  SELECT START LEVEL\r\n\r\n");

    for (var i = 1; i <= MAX_LEVEL; i++) {
        var marker = (i === cfg.startLevel) ? ATTR.menusel : ATTR.text;
        console.print(marker);
        console.print("  " + i + ". Level " + i +
            "  (delay: " + LEVELS[i-1].delay + "ms)\r\n");
    }

    console.print(ATTR.text + "\r\n  Type level number + ENTER: ");

    var input = "";
    var key;
    while (true) {
        key = console.getkey();
        if (key === "\r") break;
        if (key === "\x1b") { input = ""; break; }
        if (key >= "0" && key <= "9") {
            input += key;
            console.print(key);
        }
    }

    var num = parseInt(input);
    if (!isNaN(num) && num >= 1 && num <= MAX_LEVEL) {
        cfg.startLevel = num;
    }
}

function setupGridWidth() {
    console.clear();
    console.print(ATTR.title + "\r\n  SET GRID WIDTH (6-20)\r\n\r\n");
    console.print(ATTR.text + "  Current: " + cfg.gridWidth + "\r\n");
    console.print(ATTR.text + "  Enter new width + ENTER: ");

    var input = "";
    var key;
    while (true) {
        key = console.getkey();
        if (key === "\r") break;
        if (key === "\x1b") { input = ""; break; }
        if (key >= "0" && key <= "9") {
            input += key;
            console.print(key);
        }
    }

    var num = parseInt(input);
    if (!isNaN(num) && num >= 6 && num <= 20) {
        cfg.gridWidth = num;
    }
}

function setupGridHeight() {
    // Calculate maximum allowed height based on gridY position
    var maxHeight = MAX_SCREEN_ROW - game.gridY - 1;
    
    console.clear();
    console.print(ATTR.title + "\r\n  SET GRID HEIGHT (10-" + maxHeight + ")\r\n\r\n");
    console.print(ATTR.text + "  Current: " + cfg.gridHeight + "\r\n");
    console.print(ATTR.text + "  Max allowed: " + maxHeight + " (to fit on screen)\r\n");
    console.print(ATTR.text + "  Enter new height + ENTER: ");

    var input = "";
    var key;
    while (true) {
        key = console.getkey();
        if (key === "\r") break;
        if (key === "\x1b") { input = ""; break; }
        if (key >= "0" && key <= "9") {
            input += key;
            console.print(key);
        }
    }

    var num = parseInt(input);
    if (!isNaN(num) && num >= 10 && num <= maxHeight) {
        cfg.gridHeight = num;
    }
}

function setupMenu() {
    var running = true;
    while (running) {
        drawSetupMenu();
        var key = console.getkey().toUpperCase();

        switch (key) {
            case "1":
                setupBlockStyle();
                break;
            case "2":
                setupSpeed();
                break;
            case "3":
                setupStartLevel();
                break;
            case "4":
                cfg.useAnsiBackground = !cfg.useAnsiBackground;
                break;
            case "5":
                cfg.coloredPieces = !cfg.coloredPieces;
                break;
            case "6":
                setupGridWidth();
                break;
            case "7":
                setupGridHeight();
                break;
            case "S":
                saveConfig();
                console.print(ATTR.hilight +
                    "\r\n  Settings saved! Press any key...");
                console.getkey();
                break;
            case "Q":
            case "\x1b":
                running = false;
                break;
        }
    }
}

function showTitle() {
    console.clear();
    
    // Center the header box
    var headerLine1 = "\x01h\x01k\xC9\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xBB";
    var headerLine2 = "\x01h\x01k\xBA  \x01w  TETRIS for Synchronet BBS Software \x01k  \xBA";
    var headerLine3 = "\x01h\x01k\xC8\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xBC";
    
    var headerPad = Math.floor((console.screen_columns - getVisibleLength(headerLine1)) / 2);
    if (headerPad < 0) headerPad = 0;
    
    console.print(format("%" + headerPad + "s%s\r\n", "", headerLine1));
    console.print(format("%" + headerPad + "s%s\r\n", "", headerLine2));
    console.print(format("%" + headerPad + "s%s\r\n\r\n", "", headerLine3));

    console.print(ATTR.title);

    var lines = [
        "",
        "",
        "\x01h\x01r ##### \x01n\x01y#######\x01h #######\x01g ####  \x01c### \x01m#####",
        "\x01h\x01r   #   \x01n\x01y#      \x01h    #    \x01g#   #  \x01c#  \x01m#    ",
        "\x01h\x01r   #   \x01n\x01y#####  \x01h    #   \x01g ####   \x01c#  \x01m#####",
        "\x01h\x01r   #   \x01n\x01y#      \x01h    #   \x01g #  #   \x01c#  \x01m    #",
        "\x01h\x01r   #   \x01n\x01y#######\x01h    #    \x01g#   # \x01c### \x01m#####"
    ];

    for (var i = 0; i < lines.length; i++) {
        var pad = Math.floor((console.screen_columns - getVisibleLength(lines[i])) / 2);
        if (pad < 0) pad = 0;
        console.print(format("%" + pad + "s%s\r\n", "", lines[i]));
    }

    console.print(ATTR.subtitle);
    var sub = "\r\n\x01g";
    var pad = Math.floor((console.screen_columns - getVisibleLength(sub)) / 2);
    if (pad < 0) pad = 0;
    console.print(format("\x01h%" + pad + "s\x01n%s\r\n\r\n", "", sub));

    console.print(ATTR.text);
    var opts = [
        "P - Play Game",
        "S - Setup",
        "H - High Scores",
        "Q - Quit"
    ];

    for (var i = 0; i < opts.length; i++) {
        var p = Math.floor((console.screen_columns - getVisibleLength(opts[i])) / 2);
        if (p < 0) p = 0;
        console.print(format("%" + p + "s%s\r\n","", opts[i]));
    }

    while (true) {
        var key = console.getkey().toUpperCase();
        if (key === "P") return "play";
        if (key === "S") return "setup";
        if (key === "H") return "scores";
        if (key === "Q" || key === "\x1b") return "quit";
    }
}

function showGameOver() {
    var cx = game.gridX + game.gridWidth - 9;
    var cy = game.gridY + Math.floor(game.gridHeight / 2) - 1;

    writeAt(cx, cy,     ATTR.gameover,
        "\xC9\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xBB");
    writeAt(cx, cy + 1, ATTR.gameover,
        "\xBA    GAME  OVER! \xBA");
    writeAt(cx, cy + 2, ATTR.gameover,
        "\xC8\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xCD\xBC");

    saveHighScore();

    // Ensure we don't go below MAX_SCREEN_ROW
    var msgRow = Math.min(game.gridY + game.gridHeight + 2, MAX_SCREEN_ROW - 1);
    console.gotoxy(1, msgRow);
    console.print(ATTR.title);
    console.print(" Score: " + game.score +
                  "  Lines: " + game.lines +
                  "  Level: " + game.level + "\r\n");
    console.print(ATTR.text + " Press any key...\r\n");
    console.getkey();
}

function playGame() {
    console.clear();

    initGrid();
    game.score           = 0;
    game.lines           = 0;
    game.level           = cfg.startLevel;
    game.gameOver        = false;
    game.paused          = false;
    game.nextShape       = Math.floor(Math.random() * MAX_SHAPE);

    loadAnsiBackground();
    drawBorder();
    fillGridWithBlack();  // Fill play area with black to cover ANSI art
    drawInfoPanel();
    drawInstructions();
    updateStats();
    newBlock();

    game.lastMove = Date.now();

    while (!game.gameOver && !console.aborted) {

        var key = console.inkey(K_NOECHO | K_NOSPIN, 10);

        if (key) {
            var ku = key.toUpperCase();

            if (ku === "Q" || key === "\x1b") {
                break;
            } else if (ku === "P") {
                game.paused = !game.paused;
                if (game.paused) {
                    var pcol = game.gridX + game.gridWidth - 4;
                    var prow = game.gridY + Math.floor(game.gridHeight / 2);
                    writeAt(pcol, prow, ATTR.paused, " PAUSED ");
                } else {
                    renderGrid();
                    renderPiece(false);
                }
            } else if (!game.paused) {
                if (key === KEY_UP) {
                    rotatePiece(1);
                } else if (key === KEY_DOWN) {
                    fastDrop();
                } else if (key === KEY_LEFT) {
                    moveLeft();
                } else if (key === KEY_RIGHT) {
                    moveRight();
                } else {
                    switch (ku) {
                        case " ":
                        case "X":
                            rotatePiece(1);
                            break;
                        case "Z":
                            rotatePiece(-1);
                            break;
                        case "4":
                            moveLeft();
                            break;
                        case "6":
                            moveRight();
                            break;
                        case "2":
                        case "\r":
                            fastDrop();
                            break;
                        case "8":
                            rotatePiece(1);
                            break;
                        case "5":
                        case "0":
                            moveDown();
                            break;
                    }
                }
            }
        }

        if (!game.paused &&
            (Date.now() - game.lastMove) > getLevelDelay(game.level)) {
            moveDown();
            game.lastMove = Date.now();
        }

        mswait(5);
    }

    if (game.gameOver) {
        showGameOver();
    }
}

function main() {
    loadConfig();

    console.print("\x1b[?25l");

    var running = true;
    while (running) {
        var choice = showTitle();

        switch (choice) {
            case "play":
                playGame();
                break;
            case "setup":
                setupMenu();
                break;
            case "scores":
                showHighScores();
                break;
            case "quit":
                running = false;
                break;
        }
    }

    console.print("\x1b[?25h");
    console.print(ATTR.reset);
    console.gotoxy(1, console.screen_rows);
}

main();

// ===== END PART 4 =====
