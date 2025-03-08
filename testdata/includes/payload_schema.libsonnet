{
  type: 'object',
  properties: {
    numbers: {
      type: 'array',
      items: {
        type: 'integer',
        minimum: 1,
        maximum: 100,
      },
    },
  },
  required: ['numbers'],
}
