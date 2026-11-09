"""Entry point of `python -m ot_flex_export DB OUT.jsonl`."""

import sys

from ot_flex_export.export import main

if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
