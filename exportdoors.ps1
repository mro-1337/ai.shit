# =============================================================================
# Export-DoorGames.ps1
# Parses a Synchronet xtrn.ini file and exports [prog:*] entries into
# separate .ini files, one per section (area), each prefixed with that
# section's [sec:*] header block.
#
# Usage:
#   .\Export-DoorGames.ps1 export
#   .\Export-DoorGames.ps1 export -IniPath "C:\sbbs\ctrl\xtrn.ini" -OutputDir "C:\sbbs\exports"
# =============================================================================
param(
    [Parameter(Position = 0, Mandatory = $true)]
    [ValidateSet("export")]
    [string]$Command,

    [string]$IniPath   = "xtrn.ini",
    [string]$OutputDir = ".\DoorGameExports"
)

# ---------------------------------------------------------------------------
# 1. Verify input file
# ---------------------------------------------------------------------------
if (-not (Test-Path $IniPath)) {
    Write-Error "Cannot find input file: $IniPath"
    exit 1
}

# ---------------------------------------------------------------------------
# 2. Create output directory
# ---------------------------------------------------------------------------
if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir | Out-Null
    Write-Host "Created output directory: $OutputDir"
}

# ---------------------------------------------------------------------------
# 3. Read lines (handle both CRLF and LF)
# ---------------------------------------------------------------------------
$rawLines = Get-Content -Path $IniPath -Encoding UTF8

# ---------------------------------------------------------------------------
# 4. Parse all blocks in order — no deduplication
#    Each block: @{ Tag = "prog:GAMES:TRONJS"; Lines = @(...) }
# ---------------------------------------------------------------------------
$blocks  = [System.Collections.Generic.List[hashtable]]::new()
$current = $null

foreach ($line in $rawLines) {
    if ($line -match '^\[([^\]]+)\]') {
        if ($null -ne $current) {
            $blocks.Add($current)
        }
        $current = @{
            Tag   = $Matches[1]
            Lines = [System.Collections.Generic.List[string]]::new()
        }
        $current.Lines.Add($line.Trim())
    }
    elseif ($null -ne $current) {
        $current.Lines.Add($line.TrimEnd())
    }
}
if ($null -ne $current) { $blocks.Add($current) }

# ---------------------------------------------------------------------------
# 5. Collect sec blocks and prog blocks (preserving all duplicates)
# ---------------------------------------------------------------------------
$secBlocks   = [ordered]@{}   # area => block (last sec definition wins)
$progsByArea = [ordered]@{}   # area => List of blocks, in original file order

foreach ($block in $blocks) {
    if ($block.Tag -match '^sec:(.+)$') {
        $area = $Matches[1].ToUpper()
        $secBlocks[$area] = $block
    }
    elseif ($block.Tag -match '^prog:([^:]+):') {
        $area = $Matches[1].ToUpper()
        if (-not $progsByArea.Contains($area)) {
            $progsByArea[$area] = [System.Collections.Generic.List[hashtable]]::new()
        }
        $progsByArea[$area].Add($block)
    }
}

# ---------------------------------------------------------------------------
# 6. Write one output file per area
# ---------------------------------------------------------------------------
$filesWritten = 0

foreach ($area in $progsByArea.Keys) {
    $outPath = Join-Path (Resolve-Path $OutputDir).Path "$area.ini"
    $sb      = [System.Text.StringBuilder]::new()

    # --- Section header block ---
    if ($secBlocks.Contains($area)) {
        foreach ($ln in $secBlocks[$area].Lines) {
            [void]$sb.AppendLine($ln)
        }
    }
    else {
        # Synthesise a minimal sec block if none exists in source
        [void]$sb.AppendLine("[sec:$area]")
        [void]$sb.AppendLine("`tname=$area")
        [void]$sb.AppendLine("`tars=")
    }

    # --- All prog entries for this area, in original file order ---
    foreach ($progBlock in $progsByArea[$area]) {
        [void]$sb.AppendLine("")
        foreach ($ln in $progBlock.Lines) {
            [void]$sb.AppendLine($ln)
        }
    }

    # Write UTF-8 without BOM (Synchronet expects plain ASCII/UTF-8)
    $utf8NoBom = [System.Text.UTF8Encoding]::new($false)
    [System.IO.File]::WriteAllText($outPath, $sb.ToString(), $utf8NoBom)

    $count = $progsByArea[$area].Count
    Write-Host "  Wrote $area.ini  ($count program entries)"
    $filesWritten++
}

# ---------------------------------------------------------------------------
# 7. Summary
# ---------------------------------------------------------------------------
Write-Host ""
Write-Host "Done. $filesWritten file(s) written to: $OutputDir"