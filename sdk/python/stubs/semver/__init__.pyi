class VersionInfo:
    major: int
    minor: int
    patch: int

    def __init__(self, major, minor=0, patch=0, prerelease=None, build=None) -> None:
        ...

    def compare(self, other: VersionInfo) -> int:
        ...

    @staticmethod
    def parse(version: str) -> VersionInfo:
        ...

    def __ge__(self, other: VersionInfo) -> bool:
        ...