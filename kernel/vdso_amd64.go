// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

import _ "embed"

//go:embed vdso/vdso_amd64.so
var vdsoImage []byte
