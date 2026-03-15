/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/
// l8agent Shared Column Definitions - Conversations and Prompts tables

(function() {
    'use strict';

    window.L8Agent = window.L8Agent || {};

    const col = window.Layer8ColumnFactory;
    const enums = L8Agent.enums;
    const render = L8Agent.render;

    L8Agent.columns = {
        L8AgentConversation: [
            ...col.id('conversationId'),
            ...col.col('title', 'Title'),
            ...col.col('userId', 'User'),
            ...col.status('status', 'Status', enums.CONVO_STATUS_VALUES, render.convoStatus),
            ...col.date('createdAt', 'Created'),
            ...col.date('updatedAt', 'Updated')
        ],

        L8AgentPrompt: [
            ...col.id('promptId'),
            ...col.col('name', 'Name'),
            ...col.enum('category', 'Category', null, render.promptCategory),
            ...col.status('status', 'Status', enums.PROMPT_STATUS_VALUES, render.promptStatus),
            ...col.col('createdBy', 'Created By'),
            ...col.col('description', 'Description')
        ]
    };

    L8Agent.primaryKeys = {
        L8AgentConversation: 'conversationId',
        L8AgentPrompt: 'promptId'
    };
})();
