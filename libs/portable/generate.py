#!/usr/bin/env python3

import os
import optparse
import subprocess
from hashlib import md5, sha256
import brotli
import datetime

# 4GB maximum
length_count = 4
# encoding
encoding = 'utf-8'

# output: {path: (compressed_data, file_md5)}


def normalize(path: str) -> str:
    path = path.replace('\\', '/')
    while path.startswith('./'):
        path = path[2:]
    return path.lower()


def generate_md5_table(folder: str, level, exclude: str = None) -> dict:
    res: dict = dict()
    skip = normalize(exclude) if exclude else None
    excluded = False
    # os.curdir is the literal ".", so restoring it left us inside `folder`.
    curdir = os.getcwd()
    os.chdir(folder)
    for root, _, files in os.walk('.'):
        # remove ./
        for f in files:
            md5_generator = md5()
            full_path = os.path.join(root, f)
            if skip and normalize(full_path) == skip:
                print(f"Excluding {full_path}...")
                excluded = True
                continue
            print(f"Processing {full_path}...")
            f = open(full_path, "rb")
            content = f.read()
            content_compressed = brotli.compress(
                content, quality=level)
            md5_generator.update(content)
            md5_code = md5_generator.hexdigest().encode(encoding=encoding)
            res[full_path] = (obfuscate(content_compressed, md5_code[:8]), md5_code)
    os.chdir(curdir)
    if skip and not excluded:
        raise ValueError(f"excluded file was not found in {folder}: {exclude}")
    return res


# Payload obfuscation: each compressed entry is XORed with a SHA-256 keystream keyed by
# this constant and the file's md5 prefix. The blob layout is unchanged, so lengths stay
# identical, but a plain `brotli -d` no longer yields a runnable tree.
PACKAGE_KEY = bytes([
    0xA7, 0x3F, 0x19, 0xC4, 0x5E, 0x82, 0xD0, 0x6B,
    0x14, 0x9D, 0x27, 0xF8, 0x43, 0xB6, 0x0C, 0xE1,
    0x88, 0x52, 0x7A, 0x3D, 0xC9, 0x06, 0x5B, 0xF4,
    0x21, 0xE8, 0x9C, 0x47, 0xD3, 0x6F, 0x0A, 0x95,
])


def obfuscate(data: bytes, nonce: bytes) -> bytes:
    out = bytearray(len(data))
    counter = 0
    idx = 0
    while idx < len(data):
        block = sha256(
            PACKAGE_KEY + nonce + counter.to_bytes(8, "big")).digest()
        take = min(len(data) - idx, len(block))
        for i in range(take):
            out[idx + i] = data[idx + i] ^ block[i]
        idx += take
        counter += 1
    return bytes(out)

def write_package_metadata(md5_table: dict, output_folder: str, exe: str):
    write_blob(md5_table, os.path.join(output_folder, "data.bin"), exe)


def write_blob(md5_table: dict, output_path: str, exe: str):
    with open(output_path, "wb") as f:
        f.write("polaris".encode(encoding=encoding))
        for path in md5_table.keys():
            (compressed_data, md5_code) = md5_table[path]
            data_length = len(compressed_data)
            path = path.encode(encoding=encoding)
            # path length & path
            f.write((len(path)).to_bytes(length=length_count, byteorder='big'))
            f.write(path)
            # data length & compressed data
            f.write(data_length.to_bytes(
                length=length_count, byteorder='big'))
            f.write(compressed_data)
            # md5 code
            f.write(md5_code)
        # end
        f.write("polaris".encode(encoding=encoding))
        # executable
        f.write(exe.encode(encoding='utf-8'))
    print(f"Metadata has been written to {output_path}")

def write_app_metadata(output_folder: str):
    output_path = os.path.join(output_folder, "app_metadata.toml")
    with open(output_path, "w") as f:
        f.write(f"timestamp = {int(datetime.datetime.now().timestamp() * 1000)}\n")
    print(f"App metadata has been written to {output_path}")

def build_portable(output_folder: str, target: str):
    current_dir = os.getcwd()
    try:
        os.chdir(output_folder)
        cmd = ["cargo", "build", "--locked", "--release"]
        if target:
            cmd.extend(["--target", target])
        subprocess.run(cmd, check=True)
    finally:
        os.chdir(current_dir)

# Linux: python3 generate.py -f ../polaris-portable-packer/test -o . -e ./test/main.py
# Windows: python3 .\generate.py -f ..\rustdesk\flutter\build\windows\runner\Debug\ -o . -e ..\rustdesk\flutter\build\windows\runner\Debug\rustdesk.exe


if __name__ == '__main__':
    parser = optparse.OptionParser()
    parser.add_option("-f", "--folder", dest="folder",
                      help="folder to compress")
    parser.add_option("-o", "--output", dest="output_folder",
                      help="the root of portable packer project, default is './'")
    parser.add_option("-e", "--executable", dest="executable",
                      help="specify startup file in --folder, default is polaris.exe")
    parser.add_option("-t", "--target", dest="target",
                      help="the target used by cargo")
    parser.add_option("-l", "--level", dest="level", type="int",
                      help="compression level, default is 11, highest", default=11)
    parser.add_option("--package", dest="package",
                      help="write the per-customer blob to this path instead of "
                           "data.bin, and skip the cargo build. Injected into the "
                           "template's RDPKG resource so customizing needs no rebuild")
    parser.add_option("--exclude-exe", dest="exclude_exe", action="store_true",
                      default=False,
                      help="omit the executable from the blob, for a template whose "
                           "executable ships in the package instead")
    (options, args) = parser.parse_args()
    folder = options.folder or './polaris'
    output_folder = os.path.abspath(options.output_folder or './')

    if not options.executable:
        options.executable = 'polaris.exe' if os.path.exists(os.path.join(folder, 'polaris.exe')) else 'rustdesk.exe'
    if not options.executable.startswith(folder):
        options.executable = folder + '/' + options.executable
    # Note: the simple check `options.executable.startswith(folder)` is incorrect.
    # `python generate.py -f rustdesk -e rustdesk.exe` or `python generate.py -f rustdesk`
    # will result the print "Executable path: ..exe".
    # So we need to check if the executable is in the folder, and if so, concat again.
    if os.path.exists(os.path.join(folder, options.executable)):
        options.executable = os.path.join(folder, options.executable)
    folder_path = os.path.abspath(folder)
    exe: str = os.path.abspath(options.executable)
    try:
        in_source_folder = os.path.commonpath([folder_path, exe]) == folder_path
    except ValueError:
        in_source_folder = False
    if not in_source_folder:
        print("The executable must locate in source folder")
        exit(-1)
    exe = '.' + exe[len(folder_path):]
    print("Executable path: " + exe)
    print("Compression level: " + str(options.level))
    md5_table = generate_md5_table(
        folder, options.level, exe if options.exclude_exe else None)
    if options.package:
        write_blob(md5_table, os.path.abspath(options.package), exe)
    else:
        write_package_metadata(md5_table, output_folder, exe)
        write_app_metadata(output_folder)
        build_portable(output_folder, options.target)
