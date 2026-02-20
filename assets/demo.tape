Output demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 700
Set Padding 20
Set Theme "Catppuccin Mocha"
Set Framerate 30
Set TypingSpeed 100ms
Set WindowBar Colorful

# Start aster from the home directory for a realistic look
Type "./aster ./"
Sleep 200ms
Enter

Sleep 3s

# Navigate down through entries
Down 3
Sleep 800ms

# Enter the Downloads directory (or wherever cursor lands)
Right
Sleep 2s

# Navigate around inside
Down 2
Sleep 600ms
Down 2
Sleep 600ms

# Toggle sort by size
Type "s"
Sleep 1s

# Go back
Left
Sleep 2s

# Jump to bottom
Type "G"
Sleep 800ms

# Jump to top
Type "g"
Sleep 800ms

# Quit
Type "q"
Sleep 1s
