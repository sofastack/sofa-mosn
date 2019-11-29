/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package log

import (
	"sofastack.io/sofa-mosn/pkg/buffer"
	"sofastack.io/sofa-mosn/pkg/types"
	"sofastack.io/sofa-mosn/pkg/variable"
	"errors"
	"context"
	"fmt"
)

// RequestInfoFuncMap is a map which key is the format-key, value is the func to get corresponding string value
var (
	DefaultDisableAccessLog bool
	accessLogs              []*accesslog

	ErrLogFormatUndefined = errors.New("access log format undefined")
	ErrEmptyVarDef        = errors.New("access log format error: empty variable definition")
	ErrUnclosedVarDef     = errors.New("access log format error: unclosed variable definition")
)

const AccessLogLen = 1 << 8

func init() {
	accessLogs = []*accesslog{}
}

func DisableAllAccessLog() {
	DefaultDisableAccessLog = true
	for _, lg := range accessLogs {
		lg.logger.Toggle(true)
	}
}

// types.AccessLog
type accesslog struct {
	output  string
	entries []logEntry
	logger  *Logger
}

type logEntry struct {
	text     string
	variable variable.Variable
}

func (le logEntry) log(ctx context.Context, buf types.IoBuffer) {
	if le.text != "" {
		buf.WriteString(le.text)
	} else {
		value, err := variable.GetVariableValue(ctx, le.variable.Name())
		if err != nil {
			fmt.Println(err)
			buf.WriteString(variable.ValueNotFound)
		} else {
			buf.WriteString(value)
		}
	}
}

// NewAccessLog
func NewAccessLog(output string, format string) (types.AccessLog, error) {
	lg, err := GetOrCreateLogger(output, nil)
	if err != nil {
		return nil, err
	}

	entries, err := parseFormat(format)
	if err != nil {
		return nil, err
	}

	l := &accesslog{
		output:  output,
		entries: entries,
		logger:  lg,
	}

	if DefaultDisableAccessLog {
		lg.Toggle(true) // disable accesslog by default
	}
	// save all access logs
	accessLogs = append(accessLogs, l)

	return l, nil
}

func (l *accesslog) Log(ctx context.Context) {
	// return directly
	if l.logger.disable {
		return
	}

	buf := buffer.GetIoBuffer(AccessLogLen)
	for idx := range l.entries {
		l.entries[idx].log(ctx, buf)
	}
	buf.WriteString("\n")
	l.logger.Print(buf, true)
}

func parseFormat(format string) ([]logEntry, error) {
	if format == "" {
		return nil, ErrLogFormatUndefined
	}

	entries := make([]logEntry, 0, 8)
	varDef := false
	// last pos of '%' occur
	lastMark := -1

	for pos, ch := range format {
		switch ch {
		case '%':
			// check previous character, '\' means it is escaped
			if pos > 0 && format[pos-1] == '\\' {
				continue
			}

			// parse entry
			if pos > lastMark {
				if varDef {
					// empty variable definition: %%
					if pos == lastMark+1 {
						return nil, ErrEmptyVarDef
					}

					// var def ends, add variable
					varEntry, err := variable.AddVariable(format[lastMark+1 : pos])
					if err != nil {
						return nil, err
					}
					entries = append(entries, logEntry{variable: varEntry})
				} else {
					// ignore empty text
					if pos > lastMark+1 {
						// var def begin, add text
						textEntry := format[lastMark+1 : pos]
						entries = append(entries, logEntry{text: textEntry})
					}
				}

				lastMark = pos
			}

			// flip state
			varDef = !varDef
		default:
			continue
		}
	}

	// must ends with varDef false
	if varDef {
		return nil, ErrUnclosedVarDef
	}

	// Check remaining text part. lastMark would be equal to (length - 1) if format ends with variable def.
	formatLen := len(format)
	if lastMark < formatLen-1 {
		entries = append(entries, logEntry{text: format[lastMark+1 : formatLen]})
	}

	return entries, nil
}