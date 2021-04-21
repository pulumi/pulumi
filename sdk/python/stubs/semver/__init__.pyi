class VersionInfo:
    major: int
    minor: int
    patch: int

    def compare(self, other: VersionInfo) -> int:
        ...

    @staticmethod
    def parse(version: str) -> VersionInfo:
        ...
