/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/
// l8agent Shared Enums - Conversation and Prompt status/category enums

(function() {
    'use strict';

    const factory = window.Layer8EnumFactory;
    const { createStatusRenderer, renderEnum } = Layer8DRenderers;

    window.L8Agent = window.L8Agent || {};

    // ========================================================================
    // ENUM DEFINITIONS
    // ========================================================================

    const CONVO_STATUS = factory.create([
        ['Unspecified', null, ''],
        ['Active', 'active', 'layer8d-status-active'],
        ['Archived', 'archived', 'layer8d-status-inactive']
    ]);

    const PROMPT_CATEGORY = factory.simple([
        'Unspecified', 'General', 'Reporting', 'Workflow', 'Analysis'
    ]);

    const PROMPT_STATUS = factory.create([
        ['Unspecified', null, ''],
        ['Active', 'active', 'layer8d-status-active'],
        ['Inactive', 'inactive', 'layer8d-status-inactive']
    ]);

    const MESSAGE_ROLE = factory.simple([
        'Unspecified', 'User', 'Assistant', 'System'
    ]);

    // ========================================================================
    // EXPORT ENUMS
    // ========================================================================

    L8Agent.enums = {
        CONVO_STATUS: CONVO_STATUS.enum,
        CONVO_STATUS_VALUES: CONVO_STATUS.values,
        CONVO_STATUS_CLASSES: CONVO_STATUS.classes,
        PROMPT_CATEGORY: PROMPT_CATEGORY,
        PROMPT_STATUS: PROMPT_STATUS.enum,
        PROMPT_STATUS_VALUES: PROMPT_STATUS.values,
        PROMPT_STATUS_CLASSES: PROMPT_STATUS.classes,
        MESSAGE_ROLE: MESSAGE_ROLE
    };

    // ========================================================================
    // RENDERERS
    // ========================================================================

    const renderConvoStatus = createStatusRenderer(CONVO_STATUS.enum, CONVO_STATUS.classes);
    const renderPromptStatus = createStatusRenderer(PROMPT_STATUS.enum, PROMPT_STATUS.classes);
    const renderPromptCategory = (value) => renderEnum(value, PROMPT_CATEGORY);

    L8Agent.render = {
        convoStatus: renderConvoStatus,
        promptStatus: renderPromptStatus,
        promptCategory: renderPromptCategory,
        messageRole: (value) => renderEnum(value, MESSAGE_ROLE)
    };
})();
