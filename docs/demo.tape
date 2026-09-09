# CLI → split TUI. Run from the repository root:
#   make demo
#
# Needs contribute/pod-inspection.yaml (local draft; not shipped).
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

Type "kubeprep lab try pod-inspection.yaml"
Enter
Wait+Screen@120s /LAB SHELL/
Sleep 2s
Ctrl+G
Sleep 300ms
Type "n"
Wait+Screen@5s /kubectl get pod nginx/
Sleep 2s
Type "kubectl get pod nginx -n kubeprep-pod-inspection"
Enter
Sleep 4s
