from __future__ import annotations

import importlib.util
import io
import json
import os
import pathlib
import socket
import tempfile
import threading
import unittest
import urllib.error
from unittest import mock


SCRIPT_PATH = pathlib.Path(__file__).parents[1] / "skills" / "fuku" / "scripts" / "control.py"
SPEC = importlib.util.spec_from_file_location("control", SCRIPT_PATH)
assert SPEC is not None and SPEC.loader is not None
control = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(control)


class FakeClient:
    def __init__(self, status, services, states=None):
        self.status_result = status
        self.services_result = services
        self.states = list(states or [])
        self.actions = []

    def status(self):
        return self.status_result

    def services(self):
        return self.services_result

    def service(self, service_id):
        if len(self.states) > 1:
            return self.states.pop(0)
        return self.states[0]

    def action(self, service_id, action):
        self.actions.append((service_id, action))
        return {"id": service_id, "action": action, "status": f"{action}ing"}


def fake_api(probed, mapping):
    """Build an APIClient stand-in where each reachable base url reports the project it serves."""

    class FakeAPIClient:
        def __init__(self, base_url, token, timeout):
            self.base_url = base_url

        def live(self):
            probed.append(self.base_url)
            if self.base_url not in mapping:
                raise control.ControlError("cannot reach fuku API")

            project = mapping[self.base_url]
            if project is None:
                return {"status": "alive"}

            return {"status": "alive", "product": "fuku", "project": project}

    return FakeAPIClient


def serve_socket(test, path, status):
    """Answer one subscribe on a Unix socket with the given status banner, the way fuku does."""
    listener = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    listener.bind(str(path))
    listener.listen(4)

    stop = threading.Event()

    def serve():
        while not stop.is_set():
            try:
                connection, _ = listener.accept()
            except OSError:
                return

            with connection:
                try:
                    connection.recv(4096)
                    connection.sendall(json.dumps(status).encode("utf-8") + b"\n")
                except OSError:
                    pass

    thread = threading.Thread(target=serve, daemon=True)
    thread.start()

    def shutdown():
        stop.set()
        listener.close()
        thread.join(timeout=2)

    test.addCleanup(shutdown)


class ControlTests(unittest.TestCase):
    def test_normalize_base_url_accepts_loopback(self):
        cases = [
            "http://localhost:9876",
            "http://127.0.0.1:9876/",
            "http://127.42.0.1:1234",
            "http://[::1]:9876",
        ]

        for value in cases:
            with self.subTest(value=value):
                self.assertEqual(value.rstrip("/"), control.normalize_base_url(value))

    def test_normalize_base_url_rejects_remote_host(self):
        with self.assertRaisesRegex(control.ControlError, "loopback"):
            control.normalize_base_url("http://example.com:9876")

    def test_normalize_base_url_rejects_malformed_url(self):
        with self.assertRaisesRegex(control.ControlError, "invalid"):
            control.normalize_base_url("http://[invalid]:9876")

    def test_normalize_base_url_rejects_zero_port(self):
        with self.assertRaisesRegex(control.ControlError, "include a port"):
            control.normalize_base_url("http://127.0.0.1:0")

    def test_client_disables_http_proxies(self):
        with mock.patch.object(control.urllib.request, "build_opener") as build_opener:
            control.APIClient("http://127.0.0.1:9876", "token")

        proxy_handler = build_opener.call_args.args[0]
        self.assertIsInstance(proxy_handler, control.urllib.request.ProxyHandler)
        self.assertEqual({}, proxy_handler.proxies)

    def test_client_rejects_redirects(self):
        handler = control.NoRedirectHandler()

        with self.assertRaisesRegex(control.ControlError, "redirects"):
            handler.redirect_request(None, None, 302, "Found", {}, "http://example.com")

    def test_run_rejects_invalid_request_timeout(self):
        for value in ("0", "-1", "nan", "inf"):
            with self.subTest(value=value):
                args = control.build_parser().parse_args(
                    ["--base-url", "http://127.0.0.1:9876", "--request-timeout", value, "live"]
                )

                with self.assertRaisesRegex(control.ControlError, "--request-timeout"):
                    control.run(args)

    def test_run_rejects_invalid_wait_seconds(self):
        for value in ("0", "-1", "nan", "inf"):
            with self.subTest(value=value):
                args = control.build_parser().parse_args(
                    ["--base-url", "http://127.0.0.1:9876", "restart", "api", "--wait-seconds", value]
                )

                with self.assertRaisesRegex(control.ControlError, "--wait-seconds"):
                    control.run(args)

    def test_run_rejects_invalid_poll_interval(self):
        for value in ("-1", "nan", "inf"):
            with self.subTest(value=value):
                args = control.build_parser().parse_args(
                    ["--base-url", "http://127.0.0.1:9876", "restart", "api", "--poll-interval", value]
                )

                with self.assertRaisesRegex(control.ControlError, "--poll-interval"):
                    control.run(args)

    def test_service_by_name_requires_exact_match(self):
        services = [{"id": "1", "name": "api", "status": "running"}]

        with self.assertRaisesRegex(control.ControlError, "not found"):
            control.service_by_name(services, "ap")

    def test_start_returns_without_action_when_already_running(self):
        service = {"id": "1", "name": "api", "status": "running", "pid": 10}
        client = FakeClient({"phase": "running"}, [service])

        result = control.perform_action(client, "start", "api", 1, 0, False)

        self.assertEqual("already running", result["result"])
        self.assertEqual([], client.actions)

    def test_action_rejects_startup_phase(self):
        client = FakeClient({"phase": "startup"}, [])

        with self.assertRaisesRegex(control.ControlError, "not accepting"):
            control.perform_action(client, "restart", "api", 1, 0, False)

    def test_restart_waits_for_changed_pid(self):
        initial = {"id": "1", "name": "api", "status": "running", "pid": 10}
        states = [
            {"id": "1", "name": "api", "status": "running", "pid": 10},
            {"id": "1", "name": "api", "status": "restarting", "pid": 10},
            {"id": "1", "name": "api", "status": "running", "pid": 11},
        ]
        client = FakeClient({"phase": "running"}, [initial], states)

        result = control.perform_action(client, "restart", "api", 1, 0, False)

        self.assertEqual("completed", result["result"])
        self.assertEqual(11, result["service"]["pid"])
        self.assertEqual([("1", "restart")], client.actions)

    def test_restart_confirms_the_transition_from_revision(self):
        """A reused pid and an unchanged sampled status still resolve when the revision advanced."""
        initial = {"id": "1", "name": "api", "status": "running", "pid": 10, "revision": 7}
        states = [
            {"id": "1", "name": "api", "status": "running", "pid": 10, "revision": 7},
            {"id": "1", "name": "api", "status": "running", "pid": 10, "revision": 9},
        ]
        client = FakeClient({"phase": "running"}, [initial], states)

        result = control.perform_action(client, "restart", "api", 1, 0, False)

        self.assertEqual("completed", result["result"])
        self.assertEqual(9, result["service"]["revision"])

    def test_restart_of_a_failed_service_reports_a_repeated_failure(self):
        """An identical error after the action is a new failure once the revision advanced."""
        initial = {"id": "1", "name": "api", "status": "failed", "error": "boom", "revision": 4}
        states = [{"id": "1", "name": "api", "status": "failed", "error": "boom", "revision": 5}]
        client = FakeClient({"phase": "running"}, [initial], states)

        with self.assertRaisesRegex(control.ControlError, "api failed: boom"):
            control.perform_action(client, "restart", "api", 1, 0, False)

    def test_restart_does_not_report_a_stale_failure_as_the_result(self):
        """Before the revision advances the old error is not this action's outcome."""
        initial = {"id": "1", "name": "api", "status": "failed", "error": "boom", "revision": 4}
        states = [{"id": "1", "name": "api", "status": "failed", "error": "boom", "revision": 4}]
        client = FakeClient({"phase": "running"}, [initial], states)

        with self.assertRaisesRegex(control.ControlError, "timed out"):
            control.perform_action(client, "restart", "api", 0.05, 0.01, False)

    def test_lifecycle_moved_falls_back_when_the_instance_omits_the_revision(self):
        older = {"status": "running", "pid": 10}

        self.assertFalse(control.lifecycle_moved(older, {"status": "running", "pid": 10}, False))
        self.assertTrue(control.lifecycle_moved(older, {"status": "running", "pid": 11}, False))
        self.assertTrue(control.lifecycle_moved(older, {"status": "running", "pid": 10}, True))

    def test_stop_waits_for_stopped_state(self):
        initial = {"id": "1", "name": "storage", "status": "running", "pid": 10}
        states = [
            {"id": "1", "name": "storage", "status": "stopping", "pid": 10},
            {"id": "1", "name": "storage", "status": "stopped", "pid": 0},
        ]
        client = FakeClient({"phase": "running"}, [initial], states)

        result = control.perform_action(client, "stop", "storage", 1, 0, False)

        self.assertEqual("stopped", result["service"]["status"])

    def test_load_server_settings_applies_override_without_losing_base_values(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / "fuku.yaml").write_text(
                "server:\n  listen: '127.0.0.1:9876'\n  auth:\n    token: 'base-token'\n",
                encoding="utf-8",
            )
            (root / "fuku.override.yaml").write_text(
                "server:\n  auth:\n    token: \"local-token\" # local only\n",
                encoding="utf-8",
            )

            with mock.patch.object(control.pathlib.Path, "cwd", return_value=root):
                settings = control.load_server_settings(None)

        self.assertEqual("127.0.0.1:9876", settings["listen"])
        self.assertEqual("local-token", settings["token"])

    def test_config_paths_uses_working_directory(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / "fuku.yml").touch()

            with mock.patch.object(control.pathlib.Path, "cwd", return_value=root):
                paths = control.config_paths(None)

        self.assertEqual([root / "fuku.yml"], paths)

    def test_explicit_config_does_not_apply_default_override(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            explicit = root / "fuku.failed.yaml"
            explicit.write_text(
                "server:\n  listen: 127.0.0.1:9876\n  auth:\n    token: explicit-token\n",
                encoding="utf-8",
            )
            (root / "fuku.override.yaml").write_text(
                "server:\n  auth:\n    token: override-token\n",
                encoding="utf-8",
            )

            settings = control.load_server_settings(str(explicit))

        self.assertEqual("explicit-token", settings["token"])

    def test_run_loads_token_internally_and_does_not_return_it(self):
        token = "private-local-token"
        with tempfile.TemporaryDirectory() as directory:
            config = pathlib.Path(directory) / "fuku.yaml"
            config.write_text(
                f"server:\n  listen: 127.0.0.1:9876\n  auth:\n    token: '{token}'\n",
                encoding="utf-8",
            )
            args = control.build_parser().parse_args(["--config", str(config), "status"])

            with mock.patch.dict(os.environ, {}, clear=True), mock.patch.object(control, "APIClient") as client:
                client.return_value.live.return_value = {
                    "status": "alive",
                    "product": "fuku",
                    "project": control.project_fingerprint(pathlib.Path(directory)),
                }
                client.return_value.status.return_value = {"phase": "running"}
                result = control.run(args)

        probe = client.call_args_list[0]
        self.assertEqual(mock.call("http://127.0.0.1:9876", None, control.PROBE_TIMEOUT), probe)
        self.assertEqual(mock.call("http://127.0.0.1:9876", token, 10.0), client.call_args_list[-1])
        self.assertNotIn(token, json.dumps(result))

    def test_http_error_does_not_include_response_body(self):
        token = "private-local-token"
        client = control.APIClient("http://127.0.0.1:9876", token)
        error = urllib.error.HTTPError(
            "http://127.0.0.1:9876/api/v1/status",
            401,
            "Unauthorized",
            {},
            io.BytesIO(f'{{"error":"{token}"}}'.encode()),
        )
        client.opener = mock.Mock()
        client.opener.open.side_effect = error

        with self.assertRaises(control.ControlError) as raised:
            client.status()

        self.assertNotIn(token, str(raised.exception))

class DiscoveryTests(unittest.TestCase):
    def test_join_host_port_brackets_ipv6(self):
        self.assertEqual("127.0.0.1:1234", control.join_host_port("127.0.0.1", 1234))
        self.assertEqual("[::1]:1234", control.join_host_port("::1", 1234))

    def test_project_base_url_returns_the_port_serving_this_project(self):
        probed = []
        mine = control.project_fingerprint()

        with mock.patch.object(control, "APIClient", fake_api(probed, {"http://localhost:1236": mine})):
            result = control.project_base_url("localhost:1234", mine)

        self.assertEqual("http://localhost:1236", result)
        self.assertEqual(
            ["http://localhost:1234", "http://localhost:1235", "http://localhost:1236"],
            probed,
        )

    def test_project_base_url_reports_the_scanned_range(self):
        with mock.patch.object(control, "APIClient", fake_api([], {})):
            with self.assertRaisesRegex(control.ControlError, "ports 1234-1243"):
                control.project_base_url("localhost:1234", "a" * 16)

    def test_project_base_url_rejects_a_remote_listen_address(self):
        with self.assertRaisesRegex(control.ControlError, "loopback"):
            control.project_base_url("example.com:1234", "a" * 16)

    def test_project_base_url_names_an_instance_serving_another_project(self):
        with mock.patch.object(control, "APIClient", fake_api([], {"http://localhost:1235": "b" * 16})):
            with self.assertRaisesRegex(control.ControlError, "serve other projects"):
                control.project_base_url("localhost:1234", "a" * 16)

    def test_project_base_url_names_an_instance_that_cannot_identify_itself(self):
        with mock.patch.object(control, "APIClient", fake_api([], {"http://localhost:1235": None})):
            with self.assertRaisesRegex(control.ControlError, "upgrade fuku"):
                control.project_base_url("localhost:1234", "a" * 16)

    def test_project_fingerprint_is_stable_and_hides_the_path(self):
        with tempfile.TemporaryDirectory(dir="/tmp") as directory:
            root = pathlib.Path(directory)
            fingerprint = control.project_fingerprint(root)

            self.assertEqual(control.PROJECT_FINGERPRINT_LENGTH, len(fingerprint))
            self.assertEqual(fingerprint, control.project_fingerprint(root))
            self.assertNotIn(root.name, fingerprint)
            self.assertNotEqual(fingerprint, control.project_fingerprint(root.parent))

    def test_project_root_follows_an_explicit_config_directory(self):
        with tempfile.TemporaryDirectory(dir="/tmp") as directory:
            root = pathlib.Path(directory)
            nested = root / "examples" / "bookstore"
            nested.mkdir(parents=True)

            self.assertEqual(nested.resolve(), control.project_root(str(nested / "fuku.yaml")))
            self.assertEqual(root.resolve(), control.project_root(str(root / "fuku.yaml")))
            self.assertEqual(pathlib.Path.cwd().resolve(), control.project_root(None))

    def test_discover_fingerprints_the_explicit_config_directory(self):
        config = "server:\n  listen: \"localhost:1234\"\n  auth:\n    token: \"local\"\n"

        with self.config_dir(None) as root:
            nested = root / "examples" / "bookstore"
            nested.mkdir(parents=True)
            (nested / "fuku.yaml").write_text(config, encoding="utf-8")

            sockets = [{"profile": "minimal", "project": control.project_fingerprint(nested)}]

            with mock.patch.dict(os.environ, {}, clear=True):
                with mock.patch.object(control, "running_profiles", return_value=sockets):
                    with mock.patch.object(
                        control,
                        "probe_authenticated_instance",
                        return_value=("http://localhost:1234", {"phase": "running"}),
                    ) as probe:
                        with mock.patch.object(control, "APIClient"):
                            result = control.discover(str(nested / "fuku.yaml"))

        self.assertEqual(["minimal"], result["socket_profiles"])
        self.assertEqual([], result["other_socket_profiles"])
        self.assertEqual(
            control.project_fingerprint(nested),
            probe.call_args.args[2],
        )

    def test_running_profiles_reports_the_project_each_socket_serves(self):
        with tempfile.TemporaryDirectory(dir="/tmp") as directory:
            socket_dir = pathlib.Path(directory)

            serve_socket(self, socket_dir / "fuku-minimal.sock", {"type": "status", "project": "a" * 16})
            serve_socket(self, socket_dir / "fuku-core.sock", {"type": "status", "project": "b" * 16})

            stale = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            stale.bind(str(socket_dir / "fuku-stale.sock"))
            stale.close()

            (socket_dir / "fuku-plain-file.sock").write_text("", encoding="utf-8")
            (socket_dir / "unrelated.sock").write_text("", encoding="utf-8")

            self.assertEqual(
                [
                    {"profile": "core", "project": "b" * 16},
                    {"profile": "minimal", "project": "a" * 16},
                ],
                control.running_profiles(socket_dir),
            )

    def test_running_profiles_reports_no_project_for_an_older_instance(self):
        with tempfile.TemporaryDirectory(dir="/tmp") as directory:
            socket_dir = pathlib.Path(directory)
            serve_socket(self, socket_dir / "fuku-minimal.sock", {"type": "status", "profile": "minimal"})

            self.assertEqual([{"profile": "minimal", "project": None}], control.running_profiles(socket_dir))

    def test_running_profiles_is_empty_without_sockets(self):
        with tempfile.TemporaryDirectory(dir="/tmp") as directory:
            self.assertEqual([], control.running_profiles(pathlib.Path(directory)))

    def test_discover_separates_this_project_sockets_from_the_others(self):
        with self.config_dir("services:\n  api:\n    dir: api\n") as root:
            mine = control.project_fingerprint(root)
            sockets = [
                {"profile": "minimal", "project": mine},
                {"profile": "core", "project": "b" * 16},
                {"profile": "legacy", "project": None},
            ]

            with mock.patch.object(control, "running_profiles", return_value=sockets):
                result = control.discover(None)

        self.assertEqual(["minimal"], result["socket_profiles"])
        self.assertEqual(["core", "legacy"], result["other_socket_profiles"])
        self.assertFalse(result["api"]["reachable"])
        self.assertIn("server.listen", result["api"]["reason"])

    def test_discover_reports_the_bound_api(self):
        config = "server:\n  listen: \"localhost:1234\"\n  auth:\n    token: \"local\"\n"
        status = {"phase": "running", "profile": "minimal"}
        services = [{"name": "api", "status": "running"}]

        with self.config_dir(config) as root, mock.patch.dict(os.environ, {}, clear=True):
            with mock.patch.object(control, "running_profiles", return_value=[]):
                with mock.patch.object(
                    control,
                    "probe_authenticated_instance",
                    return_value=("http://localhost:1235", status),
                ) as probe:
                    with mock.patch.object(control, "APIClient") as client:
                        client.return_value.services.return_value = services
                        result = control.discover(None)

        self.assertEqual({"reachable": True, "base_url": "http://localhost:1235"}, result["api"])
        self.assertEqual({"status": status, "services": services}, result["instance"])
        probe.assert_called_once_with(
            "localhost:1234", "local", control.project_fingerprint(root), timeout=control.PROBE_TIMEOUT
        )

    def test_discover_reports_a_missing_config(self):
        with self.config_dir(None):
            with mock.patch.object(control, "running_profiles", return_value=[]):
                result = control.discover(None)

        self.assertFalse(result["api"]["reachable"])
        self.assertIn("was not found", result["api"]["reason"])

    def config_dir(self, config):
        """Run a block inside a temporary project directory holding an optional config."""
        test = self

        class Directory:
            def __enter__(self):
                self.previous = os.getcwd()
                self.directory = tempfile.TemporaryDirectory(dir="/tmp")
                root = pathlib.Path(self.directory.name)
                if config is not None:
                    (root / "fuku.yaml").write_text(config, encoding="utf-8")
                os.chdir(root)

                return root

            def __exit__(self, *exc_info):
                os.chdir(self.previous)
                self.directory.cleanup()

                return False

        del test

        return Directory()


class InstanceSelectionTests(unittest.TestCase):
    """Authenticated probing must select the instance configured for this project."""

    def test_probe_takes_the_nearest_port_serving_this_project(self):
        mine = control.project_fingerprint()
        mapping = {"http://localhost:1235": mine, "http://localhost:1237": mine}

        with mock.patch.object(control, "APIClient", fake_api([], mapping)):
            self.assertEqual("http://localhost:1235", control.project_base_url("localhost:1234", mine))

    def test_authenticated_probe_reaches_the_instance_serving_this_project(self):
        expected_status = {"phase": "running", "profile": "minimal"}
        mine = control.project_fingerprint()

        class FakeAPIClient:
            def __init__(self, base_url, token, timeout):
                self.base_url = base_url
                self.token = token

            def live(self):
                if self.base_url != "http://localhost:1235":
                    raise control.ControlError("cannot reach fuku API")
                return {"status": "alive", "product": "fuku", "project": mine}

            def status(self):
                return expected_status

        with mock.patch.object(control, "APIClient", FakeAPIClient):
            result = control.probe_authenticated_instance("localhost:1234", "project-token")

        self.assertEqual(("http://localhost:1235", expected_status), result)

    def test_authenticated_probe_never_offers_the_token_to_another_project(self):
        offered = []
        mine = control.project_fingerprint()

        class FakeAPIClient:
            def __init__(self, base_url, token, timeout):
                self.base_url = base_url
                if token is not None:
                    offered.append(base_url)

            def live(self):
                if self.base_url != "http://localhost:1237":
                    raise control.ControlError("cannot reach fuku API")
                return {"status": "alive", "product": "fuku", "project": "b" * 16}

            def status(self):
                return {"phase": "running", "profile": "other"}

        with mock.patch.object(control, "APIClient", FakeAPIClient):
            with self.assertRaisesRegex(control.ControlError, "serve other projects"):
                control.probe_authenticated_instance("localhost:1234", "project-token")

        self.assertEqual([], offered)
        self.assertNotEqual(mine, "b" * 16)

    def test_action_result_names_the_profile_it_reached(self):
        client = FakeClient(
            {"phase": "running", "profile": "minimal"},
            [{"id": "uuid-1", "name": "api", "status": "running", "pid": 10}],
            states=[{"id": "uuid-1", "name": "api", "status": "running", "pid": 11}],
        )

        result = control.perform_action(client, "restart", "api", 5.0, 0.0, False)

        self.assertEqual("minimal", result["profile"])
        self.assertEqual("completed", result["result"])


class LogReadTests(unittest.TestCase):
    def test_collapse_folds_consecutive_repeats(self):
        lines = [
            {"service": "api", "message": "same", "timestamp": "t1"},
            {"service": "api", "message": "same", "timestamp": "t2"},
            {"service": "api", "message": "other", "timestamp": "t3"},
            {"service": "web", "message": "same", "timestamp": "t4"},
        ]

        collapsed = control.collapse_lines(lines)

        self.assertEqual(3, len(collapsed))
        self.assertEqual(2, collapsed[0]["repeat"])
        self.assertEqual("t1", collapsed[0]["first"])
        self.assertEqual("t2", collapsed[0]["last"])
        self.assertEqual(1, collapsed[1]["repeat"])
        self.assertEqual("web", collapsed[2]["service"])

    def test_summary_ranks_by_count_and_reports_what_it_dropped(self):
        lines = [{"service": "api", "message": f"line-{index}", "timestamp": "t"} for index in range(25)]
        lines += [{"service": "api", "message": "frequent", "timestamp": "t"} for _ in range(5)]

        summary, omitted = control.summarize_lines(lines)

        self.assertEqual(control.SUMMARY_LIMIT, len(summary))
        self.assertEqual("frequent", summary[0]["message"])
        self.assertEqual(5, summary[0]["count"])
        self.assertEqual(6, omitted)

    def test_read_logs_sends_no_service_filter_without_names(self):
        client = mock.Mock()
        client.status.return_value = {"profile": "minimal"}
        client.services.return_value = [
            {"id": "id-api", "name": "api"},
            {"id": "id-web", "name": "web"},
        ]
        client.logs.return_value = {"lines": [], "tail": 100}

        result = control.read_logs(client, [], 100, None, False)

        client.logs.assert_called_once_with([], 100, None)
        self.assertEqual("minimal", result["profile"])
        self.assertEqual(0, result["total"])

    def test_read_logs_maps_requested_names_to_ids(self):
        client = mock.Mock()
        client.status.return_value = {"profile": "minimal"}
        client.services.return_value = [
            {"id": "id-api", "name": "api"},
            {"id": "id-web", "name": "web"},
        ]
        client.logs.return_value = {"lines": [], "tail": 20}

        control.read_logs(client, ["web"], 20, "5m", False)

        client.logs.assert_called_once_with(["id-web"], 20, "5m")

    def test_read_logs_rejects_a_name_outside_the_active_profile(self):
        client = mock.Mock()
        client.status.return_value = {"profile": "minimal"}
        client.services.return_value = [{"id": "id-api", "name": "api"}]

        with self.assertRaises(control.ControlError):
            control.read_logs(client, ["missing"], 20, None, False)

        client.logs.assert_not_called()

    def test_logs_request_encodes_every_service_and_bound(self):
        client = control.APIClient("http://127.0.0.1:1234", "token")

        with mock.patch.object(client, "request", return_value={"lines": []}) as request:
            client.logs(["id-a", "id-b"], 50, "2m")

        path = request.call_args[0][1]
        self.assertIn("service=id-a", path)
        self.assertIn("service=id-b", path)
        self.assertIn("tail=50", path)
        self.assertIn("since=2m", path)

    def test_logs_request_rejects_a_malformed_line_list(self):
        client = control.APIClient("http://127.0.0.1:1234", "token")

        with mock.patch.object(client, "request", return_value={"lines": "nope"}):
            with self.assertRaises(control.ControlError):
                client.logs([], 50, None)

    def test_tail_must_be_positive(self):
        parser = control.build_parser()
        args = parser.parse_args(["logs", "--tail", "0"])

        with self.assertRaises(control.ControlError):
            control.validate_numeric_options(args)


if __name__ == "__main__":
    unittest.main()
