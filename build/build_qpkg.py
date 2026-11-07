#!/usr/bin/env python3
import gzip
import io
import os
import shutil
import tarfile
from datetime import date

NAME = "FlyCast"
DISPLAY_NAME = "FlyCast"
QPKG_NAME = "FlyCast"

ROOT_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def _read_version():
    v = os.environ.get("FLYCAST_VERSION", "").strip()
    if v:
        return v
    try:
        with open(os.path.join(ROOT_DIR, "VERSION")) as fh:
            return fh.read().strip()
    except OSError:
        return "0.1.0"


VER = _read_version()
PAD = 20480

BUILD_DIR = os.path.dirname(os.path.abspath(__file__))
CONTROL_DIR = os.path.join(BUILD_DIR, "control")
PAYLOAD_DIR = os.path.join(BUILD_DIR, "payload")
TMP_DIR = os.path.join(BUILD_DIR, "tmp")
OUT_QPKG = os.path.join(ROOT_DIR, f"FlyCast_{VER}.qpkg")
BUILD_DATE = date.today().strftime("%Y%m%d")


def make_data_tar():
    os.makedirs(PAYLOAD_DIR, exist_ok=True)
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w", format=tarfile.GNU_FORMAT) as tar:
        for entry in sorted(os.listdir(PAYLOAD_DIR)):
            if entry in (".DS_Store",):
                continue
            tar.add(os.path.join(PAYLOAD_DIR, entry), arcname=entry, recursive=True)
    return buf.getvalue()


def make_control_tar_gz(tmp):
    control_dir = os.path.join(tmp, "control")
    os.makedirs(control_dir)
    for f in ("qpkg.cfg", "qinstall.sh", "package_routines"):
        shutil.copy2(os.path.join(CONTROL_DIR, f), control_dir)
    # Stamp the build version into qpkg.cfg so App Center shows the right number.
    cfg_path = os.path.join(control_dir, "qpkg.cfg")
    with open(cfg_path) as fh:
        lines = fh.read().splitlines()
    with open(cfg_path, "w") as fh:
        for line in lines:
            if line.startswith("QPKG_VER="):
                line = f'QPKG_VER="{VER}"'
            fh.write(line + "\n")
    with open(os.path.join(control_dir, "built_info"), "w") as fh:
        fh.write(f"time = {BUILD_DATE}\n")
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w", format=tarfile.GNU_FORMAT) as tar:
        for entry in sorted(os.listdir(control_dir)):
            tar.add(os.path.join(control_dir, entry), arcname=entry, recursive=False)
    raw = buf.getvalue()
    return raw, gzip.compress(raw, mtime=0)


def control_tar_member(control_gz):
    member = tarfile.TarInfo("control.tar.gz")
    member.size = len(control_gz)
    member.mtime = 0
    member.mode = 0o644
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w", format=tarfile.GNU_FORMAT) as tar:
        tar.addfile(member, io.BytesIO(control_gz))
    return buf.getvalue()


WRAPPER = """#!/bin/sh
find_base(){
HDD_MOUNT=`/sbin/getcfg Public path -f /etc/config/smb.conf`
local log_tool="/sbin/log_tool -t2 -uSystem -p127.0.0.1 -mlocalhost -a"
if [ -e "$HDD_MOUNT" ]; then
if [ -z "$QINSTALL_PATH" ]; then
QPKG_BASE=""
publicdir=`/sbin/getcfg Public path -f /etc/config/smb.conf`
if [ ! -z "$publicdir" ] && [ -d "$publicdir" ];then
publicdirp1=`/bin/echo $publicdir | /bin/cut -d "/" -f 2`
publicdirp2=`/bin/echo $publicdir | /bin/cut -d "/" -f 3`
if [ ! -z "$publicdirp1" ] && [ ! -z "$publicdirp2" ]; then
[ -d "/${publicdirp1}/${publicdirp2}/Public" ] && QPKG_BASE="/${publicdirp1}/${publicdirp2}"
fi
fi
if [ -z "$QPKG_BASE" ]; then
BASE_GROUP="/share/HDA_DATA /share/HDB_DATA /share/HDC_DATA /share/HDD_DATA /share/HDE_DATA /share/HDF_DATA /share/HDG_DATA /share/HDH_DATA /share/HDI_DATA /share/HDJ_DATA /share/HDK_DATA /share/HDL_DATA /share/MD0_DATA /share/MD1_DATA /share/MD2_DATA /share/MD3_DATA"
for datadirtest in $BASE_GROUP; do
[ -d "${datadirtest}/Public" ] && QPKG_BASE=$(dirname "${datadirtest}")
done
fi
if [ -z "$QPKG_BASE" ] ; then
echo "The Public share not found."
return 1
fi
QPKG_INSTALL_PATH="${QPKG_BASE}/.qpkg"
QPKG_DIR="${QPKG_INSTALL_PATH}/${QPKG_NAME}"
else
if [ -e "$QINSTALL_PATH" ]; then
QPKG_INSTALL_PATH="${QINSTALL_PATH}"
QPKG_DIR="${QINSTALL_PATH}/${QPKG_NAME}"
else
echo -1 > /tmp/update_process && exit 1
fi
fi
return 0
else
echo -1 > /tmp/update_process && exit 1
fi
}
wrong_arch(){
if [ -x "/usr/local/sbin/notify" ]; then
/sbin/getcfg "" "" -f /tmp/update_process >/dev/null 2>&1
/usr/local/sbin/notify send -A A039 -C C001 -M 51 -l error -t 3 "[{0}] {1} {2} install failed due to the platform is incompatible. Please use correct package for installation." "App Center" "{{DISPLAY}}" "{{VER}}"
fi
echo -1 > /tmp/update_process && exit 1
}
arch_ok(){
local cpu_arch=$(/bin/uname -m)
[ $(/usr/bin/expr match "$cpu_arch" "x86_64") -ne 0 ] && return 0
[ $(/usr/bin/expr match "$cpu_arch" "aarch64") -ne 0 ] && return 0
[ $(/usr/bin/expr match "$cpu_arch" "arm64") -ne 0 ] && return 0
return 1
}
/bin/echo "Install QNAP package on TS-NAS..."
/bin/grep "/mnt/HDA_ROOT" /proc/mounts >/dev/null 2>&1 || exit 1
arch_ok || wrong_arch
find_base
_EXTRACT_DIR="$QPKG_INSTALL_PATH/.tmp"
/bin/mkdir -p $_EXTRACT_DIR || exit 1
script_len={{SCRIPT_LEN}}
/bin/dd if="${0}" bs=$script_len skip=1 | /bin/tar -xO | /bin/tar -xzv -C $_EXTRACT_DIR || exit 1
offset=$(/usr/bin/expr $script_len + 20480)
/bin/dd if="${0}" bs=$offset skip=1 | /bin/cat | /bin/dd bs=1024 count={{DATA_COUNT}} of=$_EXTRACT_DIR/data.tar.gz 2>/dev/null || exit 1
( cd $_EXTRACT_DIR && /bin/sh qinstall.sh || echo "Installation Abort." )
/bin/rm -fr $_EXTRACT_DIR && exit 10
exit 1
"""


def build_wrapper(data_count):
    script_len_guess = None
    for _ in range(12):
        val = script_len_guess if script_len_guess is not None else len(WRAPPER) + 180
        text = WRAPPER.replace("{{DISPLAY}}", DISPLAY_NAME).replace("{{VER}}", VER)
        text = text.replace("{{SCRIPT_LEN}}", str(val)).replace("{{DATA_COUNT}}", str(data_count))
        enc = text.encode("utf-8")
        if script_len_guess == len(enc):
            return enc
        script_len_guess = len(enc)
    raise RuntimeError("script_len did not converge")


def make_tail():
    model = " ".rjust(10)
    reserved_fw = " ".rjust(50)
    name = NAME.ljust(20)[:20]
    ver = VER.ljust(10)[:10]
    flag = "QNAPQPKG  "
    tail = f"{model}{reserved_fw}{name}{ver}{flag}".encode("ascii")
    assert len(tail) == 100, len(tail)
    return tail


def add_tail_checksum(qpkg):
    size = len(qpkg)
    en = str(size * 3589 + 1000000000)[:10].encode("ascii")
    head, tail = qpkg[:-100], qpkg[-100:]
    tail = tail[:40] + en + tail[50:]
    return head + tail


def main():
    if not os.path.isdir(PAYLOAD_DIR) or not os.listdir(PAYLOAD_DIR):
        raise SystemExit("payload missing - run build/make_payload.sh first")

    shutil.rmtree(TMP_DIR, ignore_errors=True)
    os.makedirs(TMP_DIR, exist_ok=True)

    data_tar_raw = make_data_tar()
    data_gz = gzip.compress(data_tar_raw, mtime=0)
    data_count = (len(data_gz) + 1023) // 1024
    data_blob = data_gz + b"\x00" * (data_count * 1024 - len(data_gz))
    print(f"data.tar.gz: {len(data_gz)} bytes -> {data_count} x 1024B")

    _, control_gz = make_control_tar_gz(TMP_DIR)
    control_tar = control_tar_member(control_gz)
    if len(control_tar) > PAD:
        raise RuntimeError(f"control tar {len(control_tar)} > PAD {PAD}")
    print(f"control.tar: {len(control_tar)} bytes")

    wrapper = build_wrapper(data_count)
    script_len = len(wrapper)
    offset = script_len + PAD
    qpkg = wrapper + control_tar + b"\x00" * (offset - script_len - len(control_tar)) + data_blob
    qpkg = add_tail_checksum(qpkg + make_tail())

    with open(OUT_QPKG, "wb") as f:
        f.write(qpkg)
    print(f"written {OUT_QPKG}: {len(qpkg)} bytes (script_len={script_len})")

    assert qpkg.startswith(b"#!/bin/sh")
    assert qpkg[-10:] == b"QNAPQPKG  ", "tail flag missing"
    sz = len(qpkg)
    assert qpkg[-100:][40:50] == str(sz * 3589 + 1000000000)[:10].encode(), "tail checksum mismatch"
    shutil.rmtree(TMP_DIR, ignore_errors=True)
    print("build OK")


if __name__ == "__main__":
    main()
