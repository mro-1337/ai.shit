/**
 * Two-Line Inline Door Launcher
 * Reserves 2 lines and overwrites them to prevent scrolling.
 */

"use strict";

require("sbbsdefs.js", 'K_UPPER');

var doorList = [];
for (var sec in xtrn_area.sec) {
    for (var prog in xtrn_area.sec[sec].prog_list) {
        var p = xtrn_area.sec[sec].prog_list[prog];
        if (p.can_run) doorList.push({ name: p.name, code: p.code });
    }
}

var userInput = "";

// --- INITIALIZATION ---
// Print two new lines to make room, then move back up to start.
// This prevents the "pushing" effect once the loop starts.
console.putmsg("\r\n\r\n\x1b[2A"); 

while (true) {
    // 1. Search Logic
    var matches = [];
    if (userInput.length > 0) {
        var u = userInput.toUpperCase();
        for (var i = 0; i < doorList.length; i++) {
            if (doorList[i].name.toUpperCase().indexOf(u) !== -1) {
                matches.push(doorList[i]);
            }
        }
    }
    var currentMatch = matches.length > 0 ? matches[0] : null;

    // 2. Draw Top Line (Suggestions)
    console.putmsg("\r\x1b[K"); // Move to start and clear the suggestion line
    if (currentMatch) {
        console.putmsg("\x01c\x01hMatch: \x01y" + currentMatch.name + "\x01n");
    }
    
    // 3. Draw Bottom Line (Input)
    console.putmsg("\r\n\x1b[K"); // Move down one line and clear it
    console.putmsg("\x01b\x01hSearch: \x01w" + userInput);

    // ghosting in the input box
    if (userInput.length > 0 && currentMatch && currentMatch.name.toUpperCase().indexOf(userInput.toUpperCase()) === 0) {
        var ghost = currentMatch.name.substring(userInput.length);
        console.putmsg("\x01k\x01h" + ghost + "\x01n");
        // Pull cursor back to the end of actual user input
        for (var g = 0; g < ghost.length; g++) console.putmsg("\b");
    }

    // 4. Input Handling
    var key = console.getkey(K_UPPER | K_NOCRLF);

    if (key === "\r") { // Enter
        if (currentMatch) {
            // Clean up the 2 lines before launching
            console.putmsg("\r\x1b[K\x1b[1A\r\x1b[K");
            console.putmsg("\x01y\x01hLaunching " + currentMatch.name + "...\x01n\r\n");
            bbs.exec_xtrn(currentMatch.code);
            break; 
        }
    } 
    else if (key === "\x1b") { // Escape
        // Clean up the 2 lines
        console.putmsg("\r\x1b[K\x1b[1A\r\x1b[K");
        break;
    } 
    else if (key === "\b" || key === "\x7f") { // Backspace
        if (userInput.length > 0) userInput = userInput.slice(0, -1);
    } 
    else if (key.length === 1 && key >= " ") {
        if (userInput.length < 30) userInput += key;
    }

    // Move back UP to the suggestion line for the next loop iteration
    console.putmsg("\x1b[1A"); 
}