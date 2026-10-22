"""A tool. TODO: this docstring marker is documentation."""
import os
import json


class Store:
    def load(self, path):
        with open(path) as f:
            return json.load(f)

    def _save(self):
        pass


def report(v):
    return v


if __name__ == "__main__":
    report(Store().load(os.environ["STORE"]))
