# CLI → split TUI. Run from the repository root:
#   make demo
#
# Needs contribute/pod-inspection.yaml (copy from curriculum/welcome/
# if it is not already there). The training cluster should already exist
# (`make demo` runs start --prepare-only first). Creating kind inside
# this tape makes a minutes-long GIF of a spinner.
#
# VHS 0.11 cannot emit F2 or Alt+Right. Use the TUI's prefix instead:
# Ctrl+G then c (check / continue) and Ctrl+G then n (next page).
# After completion, plain n means "stay"; the prefix is what pages.

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

Type "kubeprep lab try pod-creation.yaml"
Enter
Wait+Screen@4s /LAB SHELL/
Sleep 2s
Ctrl+G
Sleep 300ms
Type "n"
Sleep 2s
Type "kubectl run nginx --image=nginx:1.27"
Enter
Sleep 2s
Type "kubectl get pods"
Enter
Sleep 2s
Type "kubectl get pod nginx -o wide"
Enter
Sleep 2s
Ctrl+G
Sleep 300ms
Type "c"
Wait+Screen@10s /SCENARIO COMPLETED/
Sleep 2s
Ctrl+G
Sleep 300ms
Type "n"
Sleep 2s
