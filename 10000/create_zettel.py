#!/usr/bin/env python
# -*- coding: utf-8 -*-

"""
create_zettel: create a directory file files to be readable by Zettelstore.
"""

import datetime
import pathlib
import shutil

ZETTEL_DIR = "zettel"

def create_zettel_dir():
    """Create a new, empty 'zettel' directory."""
    try:
        shutil.rmtree(ZETTEL_DIR)
    except OSError as exc:
        print(exc)
    result = pathlib.Path(ZETTEL_DIR)
    result.mkdir()
    return result

def copy_zettel(frompath, topath):
    """Copy the file at frompath to topath."""
    shutil.copyfile(frompath, topath)

def make_meta(metapath, title):
    """Create metafile."""
    with open(metapath, "wt") as mfile:
        print("title:", title.capitalize(), file=mfile)
        print("role: zettel", file=mfile)
        print("syntax: markdown", file=mfile)
        print("created:", datetime.datetime.now().strftime("%Y%m%d%H%M%S"), file=mfile)

def main():
    """Main function."""
    zetteldir = create_zettel_dir()
    filesdir = pathlib.Path("files.md")
    currdate = datetime.datetime(1980, 1, 1)
    for _ in range(1):
        for fpath in filesdir.iterdir():
            if not fpath.is_file():
                continue
            zid = currdate.strftime("%Y%m%d%H%M%S")
            stem = fpath.stem.replace('.', '_')
            name = stem + fpath.suffix
            copy_zettel(fpath, zetteldir.joinpath(zid + " " + name))
            make_meta(zetteldir.joinpath(zid + " " + stem), stem)
            currdate += datetime.timedelta(minutes=1)

if __name__ == '__main__':
    main()
