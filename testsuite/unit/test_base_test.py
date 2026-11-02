from pathlib import Path
import unittest
from unittest.mock import Mock, call, patch

from core.base_test import GenericTestImpl
import core.keys as keys
import core.test_constants as tconst


class GenericTestDiagnosticsTest(unittest.TestCase):
    def test_legacy_workflow_uses_explicit_compatibility_keymap(self) -> None:
        testcase = GenericTestImpl(
            test_env=Mock(), test_root=Path("fixture"), start_dir=Path("fixture"), test_dirs=[],
        )
        self.assertEqual(tconst.HOTKEY_FILE.name, "hotkeys-legacy.toml")
        self.assertEqual(testcase.spf_opts, ["-c", str(tconst.CONFIG_FILE), "--hf", str(tconst.HOTKEY_FILE)])

    def test_failure_reports_screen_captured_before_exit(self) -> None:
        environment = Mock()
        environment.spf_mgr.get_rendered_output.return_value = "fixture screen before exit"
        environment.fs_mgr.check_exists.return_value = False
        testcase = GenericTestImpl(
            test_env=environment,
            test_root=Path("fixture"),
            start_dir=Path("fixture"),
            test_dirs=[],
            validate_exists=[Path("fixture/missing.txt")],
        )

        with patch("core.base_test.time.sleep"):
            testcase.end_execution()

        self.assertEqual(
            environment.spf_mgr.mock_calls,
            [call.get_rendered_output(), call.send_special_input(keys.KEY_ESC)],
        )
        with self.assertLogs(level="DEBUG") as logs:
            self.assertFalse(testcase.validate())
        self.assertIn("fixture screen before exit", "\n".join(logs.output))


if __name__ == "__main__":
    unittest.main()
