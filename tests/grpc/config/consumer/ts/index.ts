// Copyright 2023, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as config from "@pulumi/config";

const provider = new config.Provider("provider", {
  s: pulumi.secret("foo"),
  b: true,
  i: 42,
  m: {
    fizz: "buzz",
  },
  a: [pulumi.secret("one"), "two"],
  n: {
    s: pulumi.secret("foo"),
    b: true,
    i: 42,
    m: {
      fizz: "buzz",
    },
    a: ["one", "two"],
  },
  an: [
    {
      s: pulumi.secret("bar"),
      b: false,
      i: 7,
      m: {
        fizz: "boo",
      },
      a: ["three"],
    },
  ],
});
const get = new config.Get(
  "get",
  {},
  {
    provider: provider,
  }
);
export const cfg = get.config;
export const secret0 = provider.s;
