// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as aws from "@mu/aws";
import * as mu from "mu";
import {Thumbnailer} from "./thumb";

let images = new aws.s3.Bucket("images");
let thumbnails = new aws.s3.Bucket("thumbnails");
let thumbnailer = new Thumbnailer(images, thumbnails);

