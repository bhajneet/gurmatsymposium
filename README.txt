===========================================================
GURMAT SANGEET BROADCAST SYSTEM
===========================================================

Real-time synchronization between an iPad (Controller), a Judge
Display, and OBS Studio, all over your local Wi-Fi. No internet
connection is required to run the event -- everything (styling,
fonts) is bundled inside the app, and the program data is saved
to a file next to the app as you go.

-----------------------------------------------------------
STEP 1: Download the right file
-----------------------------------------------------------
Pick ONE file for the computer that will run the show:

  macOS, Apple Silicon (M1/M2/M3/M4):  GurmatSangeet-macOS-AppleSilicon.zip
  macOS, Intel:                        GurmatSangeet-macOS-Intel.zip
  Windows, 64-bit (most PCs):          GurmatSangeet-Windows-x64.exe
  Windows, ARM64:                      GurmatSangeet-Windows-ARM64.exe

The macOS downloads are a .zip with exactly one file inside --
that's not for compression, it's because web browsers strip the
"this file is allowed to run" permission from anything they
download, and a .zip is one of the few formats that survives
that intact. Unzip it (double-click the .zip) and you're left
with one plain app file, nothing else. Windows .exe files don't
have this problem, so they download ready to run as-is.

Put it in a folder you can write to (like Desktop or Documents);
the app saves a couple of small files next to itself as you use it.

-----------------------------------------------------------
STEP 2: Run it
-----------------------------------------------------------
macOS:
  Double-click the .zip to unzip it (if your browser hasn't
  already done this automatically), then double-click the
  extracted file. macOS will warn it's from an unidentified
  developer -- right-click (or Control-click) the file, choose
  "Open", then confirm "Open" in the dialog. You only need to do
  this once.

Windows:
  Double-click the .exe file. Windows SmartScreen may warn it's
  unrecognized -- click "More info", then "Run anyway".

A console/terminal window opens showing the web address to use,
and your default browser opens automatically to the Controller
view. Keep that console window open for the rest of the event --
closing it (or pressing Ctrl+C in it) stops the server.

-----------------------------------------------------------
STEP 3: (Optional) Set a password for destructive actions
-----------------------------------------------------------
The first time you run the app, it creates a file next to itself
called "gurmat-password.txt" (empty by default). Importing a CSV
or using "Randomize" always asks for a password in-app -- with
an empty file, just leave the prompt blank and confirm.

To require a real password, close the app, open
gurmat-password.txt in a text editor, type a password, save, and
restart the app. Anyone importing a new CSV or randomizing the
order will now need to type that password.

-----------------------------------------------------------
STEP 4: Import the program CSV
-----------------------------------------------------------
Easiest option: put the day's CSV file in the same folder as the
app (alongside the .exe or macOS binary) before you run it. If
it's the only .csv file in that folder, the app imports it
automatically on startup -- nothing to click. This only happens
once, on a fresh/empty roster; it never overwrites a roster
that's already loaded.

Otherwise, on the Controller screen, tap "Import CSV" and select
the file yourself. Expected columns (header names matter more
than exact order): Group, Participant(s), Tabla accompaniment,
Performing, Raag, Taal, Notes.

Once imported, tap "Randomize" to shuffle the performance order
within each group (A, B, C, ...) -- performers are reordered
inside their own group only. The groups themselves stay in their
original order and are never merged or interleaved with each
other.

Tap the checkbox on the right of a performer's row to mark them
done for the day; completed rows dim so you can see at a glance
what's left.

-----------------------------------------------------------
STEP 5: Connect the other devices/screens
-----------------------------------------------------------
The console window shows a web address like:

  http://192.168.1.50:53211

Use it as:

- iPad Controller:
    http://<that address>/?screen=controller

- Judge / Presenter Display (supports 25% split window):
    http://<that address>/?screen=judge

- OBS Browser Source (add on the SAME computer running the app):
    URL: http://localhost:<port>/?screen=obs
    Width/Height: match your stream's canvas (1280x720, 1920x1080,
    3840x2160, etc.) -- the overlay is sized in viewport-relative
    units, so it takes up the same proportion of the frame at any
    of those resolutions. Just keep it 16:9.
    (Background is transparent automatically)

All devices must be on the same Wi-Fi network as the computer
running the app. Every screen -- Controller, Judge, and OBS --
stays in sync automatically, including across a restart, since
the program data is saved to gurmat-state.json next to the app.

-----------------------------------------------------------
STEP 6: When the event is over
-----------------------------------------------------------
Press Ctrl+C in the console window (or just close it) to stop
the server.

-----------------------------------------------------------
CSV IMPORT FORMAT
-----------------------------------------------------------
Header row required. Columns are matched by name, so they can be
reordered -- the importer looks for these keywords in the header:

  "group"        -> Group (e.g. "A (6-10)")
  "participant"  -> Participant name(s)
  "tabla"        -> Tabla accompanist
  "perform"      -> Shabad/performance title
  "raag"         -> Raag
  "taal"         -> Taal
  "note"         -> Notes shown alongside Taal on stream

Participant ages in parentheses, e.g. "Jiya Kaur (10)", are
stripped automatically for display.

-----------------------------------------------------------
FILES THE APP CREATES NEXT TO ITSELF
-----------------------------------------------------------
  gurmat-state.json      the imported roster, active performer,
                          and each performer's done/not-done flag
  gurmat-password.txt    plaintext password for Import CSV /
                          Randomize (empty = no real password set)

Both are plain text -- safe to back up, inspect, or delete
(deleting gurmat-state.json just starts you with an empty list;
deleting gurmat-password.txt makes the app recreate an empty one
next launch).
