# Operation center

Choose **Operations** in Commands, or click the Processes footer. The compact
footer stays available; no new default keybinding or persistent panel is added.

The **Operations** tab contains explicit file tasks, scans, comparisons, remote
operations, Git actions and plugins. **Background** contains automatic search,
previews, metadata, Git refresh, directory refresh checks and service status.
Repeated automatic work is grouped by kind and panel; concurrent requests remain
separate until they finish. History belongs to this session and retains at most
100 finished records across both tabs, plus every active task.

Type to filter. Use arrows, Page Up/Page Down or the mouse wheel to navigate,
Tab to change tabs, and Enter to inspect the selected task. Details include its
state, elapsed time, source/destination, available counts, partial results and
errors. Unknown totals are shown as unknown, without an extra counting scan.

The clickable controls also have keyboard equivalents:

| Control | Keyboard | Behavior |
| --- | --- | --- |
| Details | Enter | Show task details; press Enter or Escape to return |
| Cancel | Ctrl+X | Request a safe cooperative stop for the selected task |
| Clear finished | Ctrl+L | Clear session history while preserving active tasks |
| Close | Escape | Close the center without cancelling work |

Cancellation first shows **Cancel requested**. Filesystem calls and decoders may
need to finish their current call before stopping. Completed file changes remain;
cancellation is not rollback. Large/growing copies and ZIP archives are staged away
from the final filename and are not published until complete. Small copies read at
most 64 KiB before checking cancellation and exclusively creating the destination;
their bounded write completes before cancellation takes effect. Cross-volume moves retain sources
until copying succeeds and cancellation is checked before source removal.
Extraction retains completed members and removes the incomplete member when it
still belongs to the operation. Existing extraction filenames are not overwritten.
Duplicate archive member names therefore fail instead of replacing earlier output.

Directory deletion/trash operations stop between selected roots. An active OS
delete can finish or partially delete its current directory before returning.
Services and synchronous refresh checks expose details but cannot be cancelled
independently. External editors and interactive shell sessions remain controlled
by their own terminal session rather than the background-task registry.

Closing Xelif cancels registered finite workers, unblocks progress delivery, and
waits for running workers before cleaning up shared resources. Uninterruptible OS
calls can delay shutdown. Session history is not saved, resumed or retried.
