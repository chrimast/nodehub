"""Private backup for the native control plane and its SQLite state."""
import datetime
import gzip
import os
from pathlib import Path
import shutil

import tarfile


def main():
    if os.geteuid() != 0:
        raise SystemExit('Run as root to include private environment files.')
    os.umask(0o077)
    base = Path('/var/backups/harbor')
    base.mkdir(mode=0o700, parents=True, exist_ok=True)
    dest = base / datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    dest.mkdir(mode=0o700)
    state = Path('/opt/starhub-control/state')
    if not state.is_dir():
        raise SystemExit('SQLite state directory not found: '+str(state))
    with tarfile.open(dest / 'sqlite-state.tar.gz', 'w:gz') as archive:
        archive.add(state, arcname='state')
    with tarfile.open(dest / 'private-settings.tar.gz', 'w:gz') as archive:
        archive.add('/etc/harbor', arcname='etc/harbor')
    print('SQLite control backup completed:', dest.name)


if __name__ == '__main__':
    main()
