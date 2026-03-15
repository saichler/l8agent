/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/
// l8agent Shared Form Definitions - Prompt edit form

(function() {
    'use strict';

    window.L8Agent = window.L8Agent || {};

    const f = window.Layer8FormFactory;
    const enums = L8Agent.enums;

    L8Agent.forms = {
        L8AgentPrompt: f.form('Prompt Template', [
            f.section('Prompt Information', [
                ...f.text('name', 'Name', true),
                ...f.textarea('description', 'Description'),
                ...f.select('category', 'Category', enums.PROMPT_CATEGORY, true),
                ...f.select('status', 'Status', enums.PROMPT_STATUS),
                ...f.text('createdBy', 'Created By')
            ]),
            f.section('System Prompt', [
                ...f.textarea('systemPrompt', 'System Prompt Template', true)
            ])
        ])
    };
})();
