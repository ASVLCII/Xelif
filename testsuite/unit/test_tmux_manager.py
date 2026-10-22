import shlex
import unittest
from unittest.mock import Mock, patch

from core.tmux_manager import TmuxSPFManager


class TmuxSPFManagerTest(unittest.TestCase):
    def setUp(self) -> None:
        server_patch = patch("core.tmux_manager.libtmux.Server")
        self.server = server_patch.start().return_value
        self.addCleanup(server_patch.stop)
        sleep_patch = patch("core.tmux_manager.time.sleep")
        sleep_patch.start()
        self.addCleanup(sleep_patch.stop)
        self.manager = TmuxSPFManager("/tmp/build directory/spf")

    def test_explicit_start_directory_cannot_restore_a_previous_fixture(self) -> None:
        args = ["-c", "/tmp/config directory/config.toml", "--hf", "/tmp/hotkeys.toml"]
        for start_dir in ["/tmp/first fixture", "/tmp/second fixture 'with quotes'"]:
            with self.subTest(start_dir=start_dir):
                self.manager.start_spf(start_dir, args)
                call = self.server.new_session.call_args
                self.assertEqual(call.kwargs["start_directory"], start_dir)
                self.assertEqual(
                    shlex.split(call.kwargs["window_command"]),
                    [self.manager.spf_path, *args, "--", start_dir],
                )
        self.assertEqual(self.server.new_session.call_count, 2)

    def test_launch_without_start_directory_keeps_normal_launch_semantics(self) -> None:
        self.manager.start_spf()
        call = self.server.new_session.call_args
        self.assertIsNone(call.kwargs["start_directory"])
        self.assertEqual(shlex.split(call.kwargs["window_command"]), [self.manager.spf_path])

    def test_rendered_output_captures_the_active_pane(self) -> None:
        self.manager.spf_pane = Mock()
        self.manager.spf_pane.capture_pane.return_value = ["first row", "second row"]
        self.assertEqual(self.manager.get_rendered_output(), "first row\nsecond row")
        self.manager.spf_pane.capture_pane.assert_called_once_with()

    def test_rendered_output_before_start_has_no_pane(self) -> None:
        self.assertEqual(self.manager.get_rendered_output(), "[No active pane]")


if __name__ == "__main__":
    unittest.main()
