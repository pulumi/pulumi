import argparse
import pathlib
import subprocess

def parse_arguments():
    parser = argparse.ArgumentParser(
        description="A Bazel-friendly wrapper around protoc that ensures that a deterministc set of output files are generated",
    )

    parser.add_argument(
        "--protoc",
        type=str,
        help="The path to the protoc binary that should be used",
    )

    parser.add_argument(
        "--protoc-gen-grpc",
        type=str,
        help="The path to the protoc-gen-grpc binary that should be used",
    )

    parser.add_argument(
        "--protoc-gen-js",
        type=str,
        help="The path to the protoc-gen-js binary that should be used",
    )

    parser.add_argument(
        "--protoc-gen-ts",
        type=str,
        help="The path to the protoc-gen-ts binary that should be used",
    )

    parser.add_argument(
        "--output-directory",
        type=str,
        help="The directory where generated output files should be written",
    )

    parser.add_argument(
        "--descriptor-sets",
        type=str,
        action="extend",
        nargs="+",
        help="The descriptor sets that should be used",
    )

    parser.add_argument(
        "--proto-files",
        type=str,
        action="extend",
        nargs="+",
        help="The Protobuf files that should be compiled",
    )

    parser.add_argument(
        "--output-files",
        type=str,
        action="extend",
        nargs="+",
        help="The files that should be generated",
    )

    return parser.parse_args()


def main():
    args = parse_arguments()

    # The TypeScript gRPC plugin will not create .d.ts files for Protobuf files
    # that do not specify services. This is problematic for us in Bazel land,
    # since we want to predeclare all generated outputs. This wrapper works
    # around the issue by touching a fixed set of output files, which the gRPC
    # plugin will then populate a subset of with generated code. The untouched
    # files will remain empty, but that's fine.
    for output in args.output_files:
        path = pathlib.Path(output)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.touch(exist_ok=True)

    command = [
        args.protoc,

        f"--grpc_out={args.output_directory}",
        f"--js_out=import_style=commonjs,binary:{args.output_directory}",
        f"--ts_out=grpc_js:{args.output_directory}",

        f"--plugin=protoc-gen-grpc={args.protoc_gen_grpc}",
        f"--plugin=protoc-gen-js={args.protoc_gen_js}",
        f"--plugin=protoc-gen-ts={args.protoc_gen_ts}",

        f"--descriptor_set_in={':'.join(args.descriptor_sets)}",

        *args.proto_files,
    ]

    subprocess.run(command, check=True)


if __name__ == "__main__":
    main()
