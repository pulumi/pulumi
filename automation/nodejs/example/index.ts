import * as automation from '../output'
import * as uuid from 'uuid'

const test = async () => {
  const stack = 'pulumi/pulumi-test-' + uuid.v4()

  const first = await automation.stackInit(stack)
  console.log('1>', first.stdout)
  console.log('2>', first.stderr)

  let output = 'no'
  const second = await automation.upInline(() => { output = 'yes' })
  console.log('1>', second.stdout)
  console.log('2>', second.stderr)

  console.log(output)
}

test()
