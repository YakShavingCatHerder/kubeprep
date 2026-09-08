# CLI → split TUI. Run from the repository root:
#   make demo
#
# The training cluster should already exist (`make install` then
# `kubeprep start --track=beginner --prepare-only`). Creating kind inside
# this tape makes a minutes-long GIF of a spinner.

Output docs/demo.gif

Set Shell "zsh"
Set FontSize 14
Set Width 1500
Set Height 850
Set Padding 16
Set WindowBar Colorful
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms
Set Framerate 24
Set PlaybackSpeed 0.5

Hide
Type `export PATH="$PWD/bin:$PATH"`
Enter
Ctrl+L
Show

Type "kubeprep lab try test-lab.yaml"
Enter
Wait+Screen@120s /LAB SHELL/
Sleep 4s
Type "kubectl get pods -n kubeprep-beginner"
Enter
Sleep 3s
