"""Rewrites the server configs: etcd service discovery becomes direct addresses.

In this build etcd only helps services find each other. They all live in one
container on known ports, so a registry is one moving part too many: go-zero can
use direct Endpoints.

Telemetry is off unless a collector is given: with none running, every service
logs a trace export error once a second and floods the output. Point it at one
and the whole path of a request becomes visible instead of being reassembled
from logs by hand.

Every service also gets a DevServer: pprof and metrics on its own port. Claims
about where the time goes should come from a profile, not from a duration field
in a log line.

Usage: python3 deploy/etc/rewrite-configs.py <source> <output> [--telemetry host:port]
"""
import re
import sys
from pathlib import Path

# etcd service key -> the address it listens on (all processes share one container)
ADDRESSES = {
    "service.authsession": "127.0.0.1:20450",
    "bff.bff": "127.0.0.1:20010",
    "service.biz_service": "127.0.0.1:20020",
    "service.dfs": "127.0.0.1:20640",
    "interface.gateway": "127.0.0.1:20110",
    "service.idgen": "127.0.0.1:20660",
    "service.media": "127.0.0.1:20650",
    "messenger.msg": "127.0.0.1:20030",
    "interface.session": "127.0.0.1:20120",
    "service.status": "127.0.0.1:20670",
    "messenger.sync": "127.0.0.1:20420",
}

# An Etcd block: indentation, an optional dash (when it is an item of a client
# list), Hosts with the address list and Key with the service name
ETCD_BLOCK = re.compile(
    r"^(?P<indent>[ ]*)(?P<dash>-[ ]+)?Etcd:\n"
    r"(?:^[ ]+Hosts:\n(?:^[ ]+-[ ]*\S+\n)+)"
    r"^[ ]+Key:[ ]*(?P<key>\S+)\n",
    re.M,
)

TELEMETRY_BLOCK = re.compile(r"^Telemetry:\n(?:^[ ]+\S.*\n)+", re.M)

# The service port. Listening on every interface is not an option: the service
# announces itself with the address it sees locally (the container IP), which then
# does not match the direct addresses from the configs and messages cannot find
# their way back. MTProto client ports (10443 and the rest) are configured
# separately and are not affected.
LISTEN_ON = re.compile(r"^ListenOn:[ ]*0\.0\.0\.0:(?P<port>\d+)[ ]*$", re.M)

# Object store settings: without an address the server writes files to disk.
MINIO_BLOCK = re.compile(
    r"^Minio:\n"
    r"[ ]+Endpoint:[ ]*\S+\n"
    r"[ ]+AccessKeyID:[ ]*\S+\n"
    r"[ ]+SecretAccessKey:[ ]*\S+\n"
    r"[ ]+UseSSL:[ ]*\S+\n",
    re.M,
)

# The queue client: Topic and Brokers. We swap them for direct service addresses,
# so work travels by a call rather than through Kafka (see pkg/queue in the server
# fork).
QUEUE_CLIENT = re.compile(
    r"^(?P<name>SyncClient|InboxClient|BotSyncClient|PushClient):\n"
    r"[ ]+Topic:[ ]*\"(?P<topic>[^\"]+)\"\n"
    r"[ ]+Brokers:\n(?:[ ]+-[ ]*\S+\n)+",
    re.M,
)

# The queue consumer: without Brokers the service does not start it and listens on gRPC only
QUEUE_CONSUMER = re.compile(
    r"^(?P<name>\w*Consumer):\n(?:^[ ]+\S.*\n)+",
    re.M,
)

# Where the work that used to go into the queue is routed now
QUEUE_TARGETS = {
    "Sync-T": "127.0.0.1:20420",   # sync
    "Inbox-T": "127.0.0.1:20030",  # inbox lives inside the msg process
}


# pprof and metrics for a service, on a port derived from the one it already
# listens on so two services can never land on the same one.
def dev_server_block(text: str) -> str:
    match = re.search(r"^ListenOn:[ ]*\S+:(\d+)[ ]*$", text, re.M)
    if not match:
        return ""
    return (f"DevServer:\n"
            f"  Enabled: true\n"
            f"  Host: 127.0.0.1\n"
            f"  Port: {int(match.group(1)) + 10000}\n"
            f"  EnablePprof: true\n")


def telemetry_block(name: str, endpoint: str) -> str:
    return (f"Telemetry:\n"
            f"  Name: {name}\n"
            f"  Endpoint: {endpoint}\n"
            f"  Sampler: 1.0\n"
            f"  Batcher: otlpgrpc\n")


def rewrite(text: str, telemetry: str = "") -> str:
    def replace(match):
        indent = match.group("indent")
        dash = match.group("dash")
        key = match.group("key")
        address = ADDRESSES.get(key)
        if address is None:
            raise SystemExit(f"unknown service in the config: {key}")
        if not indent and not dash:
            # A top-level Etcd block registers the server in the registry.
            # Without a registry it is pointless: clients use direct addresses.
            return ""
        # Key stays: the code uses it to match a method with the right client
        # (see NewBFFProxyClients). Without Hosts the registry is unused —
        # go-zero sees direct addresses and goes by them.
        # Hosts stays an empty list: go-zero treats the field as mandatory, and an
        # empty list reads as "no registry", so the client uses the addresses.
        if dash:
            # a list item: the key shifts by the dash width, nested lines shift further
            return (f"{indent}- Etcd:\n{indent}      Hosts: []\n{indent}      Key: {key}\n"
                    f"{indent}  Endpoints:\n{indent}    - {address}\n")
        return (f"{indent}Etcd:\n{indent}  Hosts: []\n{indent}  Key: {key}\n"
                f"{indent}Endpoints:\n{indent}  - {address}\n")

    name_match = re.search(r"^Name:[ ]*(\S+)[ ]*$", text, re.M)
    name = name_match.group(1) if name_match else "ice9"
    text = ETCD_BLOCK.sub(replace, text)
    text = TELEMETRY_BLOCK.sub(telemetry_block(name, telemetry) if telemetry else "", text)
    text = LISTEN_ON.sub(lambda m: f"ListenOn: 127.0.0.1:{m.group('port')}", text)

    def replace_queue(match):
        address = QUEUE_TARGETS.get(match.group("topic"))
        if address is None:
            raise SystemExit(f"unknown queue topic: {match.group('topic')}")
        return f"{match.group('name')}:\n  Endpoints:\n    - {address}\n"

    text = MINIO_BLOCK.sub("Minio:\n  Dir: /app/data/files\n", text)
    text = QUEUE_CLIENT.sub(replace_queue, text)
    text = QUEUE_CONSUMER.sub("", text)
    if "DevServer:" not in text:
        text = text.rstrip("\n") + "\n\n" + dev_server_block(text)
    return text


def main(source: Path, target: Path, telemetry: str = ""):
    target.mkdir(parents=True, exist_ok=True)
    for path in sorted(source.glob("*.yaml")):
        result = rewrite(path.read_text(), telemetry)
        (target / path.name).write_text(result)
        print(f"[config] {path.name}")
    print(f"[config] done: {len(list(source.glob('*.yaml')))} files")


if __name__ == "__main__":
    args = sys.argv[1:]
    collector = ""
    if "--telemetry" in args:
        index = args.index("--telemetry")
        collector = args[index + 1]
        del args[index:index + 2]
    if len(args) != 2:
        raise SystemExit(__doc__)
    main(Path(args[0]), Path(args[1]), collector)
